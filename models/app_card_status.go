package models

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/sweetrpg/admin-api/constants"
	"github.com/sweetrpg/mongodb.go/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AppCardStatusScopeType identifies what an app-card-status record applies to.
type AppCardStatusScopeType string

const (
	AppCardStatusScopeService AppCardStatusScopeType = "service"
)

func (s AppCardStatusScopeType) valid() bool {
	switch s {
	case AppCardStatusScopeService:
		return true
	default:
		return false
	}
}

// AppCardStatus is the MongoDB document for a service-scoped app card status.
// It controls the enabled/disabled state and display label for a service's app card.
type AppCardStatus struct {
	ID         primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	ScopeType  AppCardStatusScopeType `bson:"scope_type" json:"scope_type"`
	ScopeValue string                 `bson:"scope_value" json:"scope_value"`
	Enabled    bool                   `bson:"enabled" json:"enabled"`
	Label      string                 `bson:"label" json:"label"`
	CreatedAt  time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt  time.Time              `bson:"updated_at" json:"updated_at"`
}

// Validate checks that the record satisfies the invariants required by the spec:
// scope_value must not be empty, label must not be empty and must not exceed 24 runes.
func (a *AppCardStatus) Validate() error {
	if !a.ScopeType.valid() {
		return errors.New("scope_type must be service")
	}
	if a.ScopeValue == "" {
		return errors.New("scope_value is required")
	}
	if a.Label == "" {
		return errors.New("label is required")
	}
	if utf8.RuneCountInString(a.Label) > 24 {
		return errors.New("label must not exceed 24 characters")
	}
	return nil
}

// EnsureAppCardStatusIndexes creates a unique compound index on (scope_type,
// scope_value) so at most one record exists per scope. Safe to call on every
// startup - CreateOne is idempotent for an identical index definition.
func EnsureAppCardStatusIndexes(ctx context.Context) error {
	collection := database.Db.Collection(constants.AppCardStatusCollection)

	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "scope_type", Value: 1}, {Key: "scope_value", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}
