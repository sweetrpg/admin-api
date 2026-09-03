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

// MaintenanceScopeType identifies what a maintenance-mode record applies to. Unlike
// BannerMessage's ScopeType, maintenance mode has no page scope - it's app/platform
// granularity only.
type MaintenanceScopeType string

const (
	MaintenanceScopePlatform MaintenanceScopeType = "platform"
	MaintenanceScopeService  MaintenanceScopeType = "service"
)

func (s MaintenanceScopeType) valid() bool {
	switch s {
	case MaintenanceScopePlatform, MaintenanceScopeService:
		return true
	default:
		return false
	}
}

// MaintenanceMode is the MongoDB document for a platform-wide or service-scoped
// maintenance state. Unlike BannerMessage, it's a durable on/off record - one per
// scope, edited in place rather than accumulated - with enabled as the explicit gate;
// starts_at/ends_at are informational for display only.
type MaintenanceMode struct {
	ID          primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	ScopeType   MaintenanceScopeType `bson:"scope_type" json:"scope_type"`
	ScopeValue  string               `bson:"scope_value" json:"scope_value"`
	Enabled     bool                 `bson:"enabled" json:"enabled"`
	StartsAt    time.Time            `bson:"starts_at" json:"starts_at"`
	EndsAt      *time.Time           `bson:"ends_at,omitempty" json:"ends_at,omitempty"`
	Label       string               `bson:"label" json:"label"`
	Description string               `bson:"description" json:"description"`
	// Platform audit fields (PADR-0001); hard-delete record (PADR-0027), no deleted_* pair.
	CreatedBy string    `bson:"created_by" json:"created_by"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedBy string    `bson:"updated_by" json:"updated_by"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// Validate checks that the record satisfies the invariants required by the spec: a
// known scope type, a scope value present exactly when the scope requires one, a
// required starts_at, and required label/description.
func (m *MaintenanceMode) Validate() error {
	if !m.ScopeType.valid() {
		return errors.New("scope_type must be one of: platform, service")
	}
	if m.ScopeType == MaintenanceScopePlatform && m.ScopeValue != "" {
		return errors.New("scope_value must be empty for platform scope")
	}
	if m.ScopeType == MaintenanceScopeService && m.ScopeValue == "" {
		return errors.New("scope_value is required for service scope")
	}
	if m.StartsAt.IsZero() {
		return errors.New("starts_at is required")
	}
	if m.Label == "" {
		return errors.New("label is required")
	}
	if m.Description == "" {
		return errors.New("description is required")
	}
	if m.EndsAt != nil && m.EndsAt.Before(m.StartsAt) {
		return errors.New("ends_at must be after starts_at")
	}
	return nil
}

// EnsureMaintenanceModeIndexes creates a unique compound index on (scope_type,
// scope_value) so at most one record exists per scope - a create for an existing
// scope updates in place instead of accumulating duplicates. Safe to call on every
// startup - CreateOne is idempotent for an identical index definition.
func EnsureMaintenanceModeIndexes(ctx context.Context) error {
	collection := database.Db.Collection(constants.MaintenanceModeCollection)

	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "scope_type", Value: 1}, {Key: "scope_value", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}
