// Command backfill-audit-fields stamps the platform audit fields (PADR-0001) on admin-api's
// pre-convention documents: updated_by on banners, and created_by / updated_by on
// maintenance_modes. created_at / updated_at are already present on both. No acting user was
// recorded historically, so a missing *_by is set to the "system" sentinel; on a banner an
// existing created_by seeds updated_by instead.
//
// banners and maintenance_modes are hard-delete operational records (PADR-0027) - there is no
// deleted_at / deleted_by pair to backfill.
//
// Idempotent - a document that already has updated_by is skipped. Dry run by default.
//
//	go run ./cmd/backfill-audit-fields          # report counts, write nothing
//	go run ./cmd/backfill-audit-fields -apply   # perform the writes
package main

import (
	"context"
	"flag"

	"github.com/joho/godotenv"
	"github.com/sweetrpg/admin-api/constants"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/mongodb.go/database"
	"go.mongodb.org/mongo-driver/bson"
)

// systemActor is model-core's SystemActor value (PADR-0001). Inlined - admin-api has no
// model-core.go dependency and this is the only place that needs the constant.
const systemActor = "system"

func main() {
	apply := flag.Bool("apply", false, "perform writes (default: dry run)")
	flag.Parse()

	_ = godotenv.Load(".env")
	logging.Init()

	database.SetupDatabase()
	defer database.TeardownDatabase()

	ctx := context.Background()
	mode := "DRY RUN"
	if *apply {
		mode = "APPLY"
	}

	needsUpdatedBy := bson.D{{Key: "$or", Value: bson.A{
		bson.D{{Key: "updated_by", Value: bson.D{{Key: "$exists", Value: false}}}},
		bson.D{{Key: "updated_by", Value: ""}},
	}}}

	// banners: updated_by <- created_by if set, else "system".
	banners := stamp(ctx, constants.BannerCollection, needsUpdatedBy, *apply, func(d bson.Raw) bson.D {
		by := systemActor
		if s, ok := d.Lookup("created_by").StringValueOK(); ok && s != "" {
			by = s
		}
		return bson.D{{Key: "updated_by", Value: by}}
	})

	// maintenance_modes: created_by / updated_by <- "system" (no acting user recorded).
	modes := stamp(ctx, constants.MaintenanceModeCollection, needsUpdatedBy, *apply, func(bson.Raw) bson.D {
		return bson.D{
			{Key: "created_by", Value: systemActor},
			{Key: "updated_by", Value: systemActor},
		}
	})

	logging.Logger.Info("backfill-audit-fields done", "mode", mode, "banners", banners, "maintenance_modes", modes)
}

// stamp applies set(doc) to every doc in coll matching filter. Returns the count touched (or the
// count that would be touched, on a dry run).
func stamp(ctx context.Context, coll string, filter bson.D, apply bool, set func(bson.Raw) bson.D) int {
	cur, err := database.Db.Collection(coll).Find(ctx, filter)
	if err != nil {
		logging.Logger.Error("query failed", "collection", coll, "error", err.Error())
		return 0
	}
	var docs []bson.Raw
	if err := cur.All(ctx, &docs); err != nil {
		logging.Logger.Error("cursor read failed", "collection", coll, "error", err.Error())
		return 0
	}
	n := 0
	for _, d := range docs {
		if !apply {
			n++
			continue
		}
		if _, err := database.Db.Collection(coll).UpdateOne(ctx,
			bson.D{{Key: "_id", Value: d.Lookup("_id")}},
			bson.D{{Key: "$set", Value: set(d)}}); err != nil {
			logging.Logger.Error("update failed", "collection", coll, "error", err.Error())
			continue
		}
		n++
	}
	return n
}
