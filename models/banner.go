// Package models defines the persisted document shapes for admin-api.
package models

import (
	"context"
	"errors"
	"time"

	"github.com/sweetrpg/admin-api/constants"
	"github.com/sweetrpg/mongodb.go/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ScopeType identifies what a banner message applies to.
type ScopeType string

const (
	ScopePlatform ScopeType = "platform"
	ScopeService  ScopeType = "service"
	ScopePage     ScopeType = "page"
)

// Severity ranks how urgent a banner message is.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// severityRank orders severities most-severe-first; lower ranks first.
var severityRank = map[Severity]int{
	SeverityCritical: 0,
	SeverityWarning:  1,
	SeverityInfo:     2,
}

// Rank returns the sort-order weight for this severity (0 = most severe).
// Unknown severities sort last.
func (s Severity) Rank() int {
	if r, ok := severityRank[s]; ok {
		return r
	}
	return len(severityRank)
}

func (s ScopeType) valid() bool {
	switch s {
	case ScopePlatform, ScopeService, ScopePage:
		return true
	default:
		return false
	}
}

func (s Severity) valid() bool {
	switch s {
	case SeverityInfo, SeverityWarning, SeverityCritical:
		return true
	default:
		return false
	}
}

// BannerMessage is the MongoDB document for a platform-wide, service-scoped, or
// page-scoped admin banner.
type BannerMessage struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ScopeType  ScopeType          `bson:"scope_type" json:"scope_type"`
	ScopeValue string             `bson:"scope_value" json:"scope_value"`
	Severity   Severity           `bson:"severity" json:"severity"`
	Message    string             `bson:"message" json:"message"`
	StartsAt   *time.Time         `bson:"starts_at,omitempty" json:"starts_at,omitempty"`
	ExpiresAt  time.Time          `bson:"expires_at" json:"expires_at"`
	// Platform audit fields (PADR-0001); hard-delete record (PADR-0027), no deleted_* pair.
	CreatedBy string    `bson:"created_by" json:"created_by"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedBy string    `bson:"updated_by" json:"updated_by"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// Validate checks that the banner satisfies the invariants required by the spec:
// a known scope type, a scope value present exactly when the scope requires one, a
// known severity, a non-empty message, and a required expires_at.
func (b *BannerMessage) Validate() error {
	if !b.ScopeType.valid() {
		return errors.New("scope_type must be one of: platform, service, page")
	}
	if b.ScopeType == ScopePlatform && b.ScopeValue != "" {
		return errors.New("scope_value must be empty for platform scope")
	}
	if b.ScopeType != ScopePlatform && b.ScopeValue == "" {
		return errors.New("scope_value is required for service and page scope")
	}
	if !b.Severity.valid() {
		return errors.New("severity must be one of: info, warning, critical")
	}
	if b.Message == "" {
		return errors.New("message is required")
	}
	if b.ExpiresAt.IsZero() {
		return errors.New("expires_at is required")
	}
	if b.StartsAt != nil && b.StartsAt.After(b.ExpiresAt) {
		return errors.New("starts_at must be before expires_at")
	}
	return nil
}

// EnsureIndexes creates the indexes admin-api relies on: a MongoDB TTL index on
// expires_at that physically removes expired documents, and a compound index on
// (scope_type, scope_value) to keep the scoped-query path fast. Safe to call on every
// startup - CreateOne is idempotent for an identical index definition.
func EnsureIndexes(ctx context.Context) error {
	collection := database.Db.Collection(constants.BannerCollection)

	_, err := collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
		{
			Keys: bson.D{{Key: "scope_type", Value: 1}, {Key: "scope_value", Value: 1}},
		},
	})
	return err
}
