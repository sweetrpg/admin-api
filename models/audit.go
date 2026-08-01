package models

import (
	"context"
	"time"

	"github.com/sweetrpg/admin-api/constants"
	"github.com/sweetrpg/mongodb.go/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AuditStatus tracks whether the mutation an AdminActionAuditLog record
// covers actually completed.
type AuditStatus string

const (
	AuditAttempted AuditStatus = "attempted"
	AuditSucceeded AuditStatus = "succeeded"
	AuditFailed    AuditStatus = "failed"
)

// AdminActionAuditLog attributes a write-route mutation to the acting user
// who made it. Written *before* the mutation is attempted (status
// "attempted") and updated *after* it completes - see RecordAuditAttempt and
// CompleteAudit. If the "before" write fails, the mutation must not be
// performed: an audit trail that can silently fail to exist is not an audit
// trail, mirroring users-api's AdminActionAuditLog/performAudited pattern.
// bson tags on every field but ID/Status use omitempty: database.Update
// replaces the stored document with a $set of every marshaled field of
// whatever doc it's given, so CompleteAudit - which only ever populates
// Status/CompletedAt/ErrorMessage on top of an existing record's ID - must
// not marshal its unset fields as empty values, or it would overwrite the
// original ActingUserSub/Action/AttemptedAt with blanks.
type AdminActionAuditLog struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ActingUserSub string             `bson:"acting_user_sub,omitempty" json:"acting_user_sub"`
	Action        string             `bson:"action,omitempty" json:"action"`
	ResourceID    string             `bson:"resource_id,omitempty" json:"resource_id,omitempty"`
	Detail        string             `bson:"detail,omitempty" json:"detail,omitempty"`
	Status        AuditStatus        `bson:"status" json:"status"`
	AttemptedAt   time.Time          `bson:"attempted_at,omitempty" json:"attempted_at"`
	CompletedAt   *time.Time         `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
	ErrorMessage  string             `bson:"error_message,omitempty" json:"error_message,omitempty"`
}

// RecordAuditAttempt writes the "attempted" audit record for a write-route
// mutation before the mutation itself runs. Returns the record's ID for a
// later CompleteAudit call. Callers must not perform the mutation if this
// returns an error.
func RecordAuditAttempt(actingUserSub, action, resourceID, detail string) (primitive.ObjectID, error) {
	entry := &AdminActionAuditLog{
		ActingUserSub: actingUserSub,
		Action:        action,
		ResourceID:    resourceID,
		Detail:        detail,
		Status:        AuditAttempted,
		AttemptedAt:   time.Now().UTC(),
	}
	return database.Insert(constants.AdminActionAuditLogCollection, entry)
}

// CompleteAudit updates an audit record to its final status after the
// mutation it covers has run. Best-effort: a failure here doesn't undo a
// mutation that already happened, so callers should log a warning rather
// than fail the request on error.
func CompleteAudit(id primitive.ObjectID, status AuditStatus, errMessage string) error {
	now := time.Now().UTC()
	update := &AdminActionAuditLog{
		ID:           id,
		Status:       status,
		CompletedAt:  &now,
		ErrorMessage: errMessage,
	}
	_, _, err := database.Update(constants.AdminActionAuditLogCollection, id, update)
	return err
}

// EnsureAuditIndexes creates the TTL index on admin_action_audit_logs: audit
// records are retained for 1 year (longer than banners' expiry-driven TTL -
// see design.md's Open Questions), independent of any per-document field.
// Safe to call on every startup - CreateOne is idempotent for an identical
// index definition.
func EnsureAuditIndexes(ctx context.Context) error {
	collection := database.Db.Collection(constants.AdminActionAuditLogCollection)

	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "attempted_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(365 * 24 * 60 * 60),
	})
	return err
}
