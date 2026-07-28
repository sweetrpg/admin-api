package server

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/admin-api/constants"
	"github.com/sweetrpg/admin-api/models"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/mongodb.go/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func setupBannerHandlers(g *gin.Engine) {
	logging.Logger.Info("Setting up banner endpoint handlers...")

	g.POST("/banners", createBanner)
	g.GET("/banners", listBanners)
	g.PUT("/banners/:id", updateBanner)
	g.DELETE("/banners/:id", deleteBanner)
}

// createBannerRequest is the payload accepted by POST /banners.
type createBannerRequest struct {
	ScopeType  models.ScopeType `json:"scope_type" binding:"required"`
	ScopeValue string           `json:"scope_value"`
	Severity   models.Severity  `json:"severity" binding:"required"`
	Message    string           `json:"message" binding:"required"`
	StartsAt   *time.Time       `json:"starts_at"`
	ExpiresAt  time.Time        `json:"expires_at" binding:"required"`
	CreatedBy  string           `json:"created_by" binding:"required"`
}

// updateBannerRequest is the payload accepted by PUT /banners/:id. created_by and
// created_at are immutable once a banner exists, so they aren't part of the update
// payload.
type updateBannerRequest struct {
	ScopeType  models.ScopeType `json:"scope_type" binding:"required"`
	ScopeValue string           `json:"scope_value"`
	Severity   models.Severity  `json:"severity" binding:"required"`
	Message    string           `json:"message" binding:"required"`
	StartsAt   *time.Time       `json:"starts_at"`
	ExpiresAt  time.Time        `json:"expires_at" binding:"required"`
}

// Create a banner message.
//
//	@Summary		Create a banner message
//	@Description	Creates a new admin banner message. expires_at is required; a banner without one is rejected.
//	@Tags			banners
//	@Accept			json
//	@Produce		json
//	@Param			banner	body		createBannerRequest	true	"Banner message"
//	@Success		201		{object}	models.BannerMessage
//	@Failure		400		{object}	apiv.ErrorVO
//	@Failure		500		{object}	apiv.ErrorVO
//	@Router			/banners [post]
func createBanner(c *gin.Context) {
	var req createBannerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	now := time.Now().UTC()
	banner := &models.BannerMessage{
		ScopeType:  req.ScopeType,
		ScopeValue: req.ScopeValue,
		Severity:   req.Severity,
		Message:    req.Message,
		StartsAt:   req.StartsAt,
		ExpiresAt:  req.ExpiresAt,
		CreatedBy:  req.CreatedBy,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := banner.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "validation_failed", Message: err.Error()})
		return
	}

	_, span := otel.Tracer("banners").Start(c.Request.Context(), "create-banner")
	id, err := database.Insert(constants.BannerCollection, banner)
	span.End()
	if err != nil {
		logging.Logger.Error("Failed to insert banner", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "insert_failed", Message: "failed to create banner"})
		return
	}
	banner.ID = id

	c.JSON(http.StatusCreated, banner)
}

// List active banner messages for one or more scopes.
//
//	@Summary		List active banners for a scope
//	@Description	Returns active (started, not expired) banners matching any of the requested scopes, most-severe-first. Platform-scoped banners are always included. scope values look like "platform", "service:catalog", or "page:/catalog/browse".
//	@Tags			banners
//	@Produce		json
//	@Param			scope	query		[]string	true	"Repeated scope filter, e.g. scope=platform&scope=service:catalog"
//	@Success		200		{array}		models.BannerMessage
//	@Failure		400		{object}	apiv.ErrorVO
//	@Failure		500		{object}	apiv.ErrorVO
//	@Router			/banners [get]
func listBanners(c *gin.Context) {
	scopes := c.QueryArray("scope")
	if len(scopes) == 0 {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "missing_scope", Message: "at least one scope query param is required"})
		return
	}

	var serviceValues, pageValues []string
	for _, raw := range scopes {
		scopeType, scopeValue := parseScope(raw)
		switch scopeType {
		case models.ScopeService:
			serviceValues = append(serviceValues, scopeValue)
		case models.ScopePage:
			pageValues = append(pageValues, scopeValue)
		}
		// models.ScopePlatform requires no extra collection - platform banners are
		// always included below.
	}

	now := time.Now().UTC()
	activeFilter := bson.D{
		{Key: "expires_at", Value: bson.D{{Key: "$gt", Value: now}}},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "starts_at", Value: bson.D{{Key: "$exists", Value: false}}}},
			bson.D{{Key: "starts_at", Value: nil}},
			bson.D{{Key: "starts_at", Value: bson.D{{Key: "$lte", Value: now}}}},
		}},
	}

	scopeOr := bson.A{bson.D{{Key: "scope_type", Value: models.ScopePlatform}}}
	if len(serviceValues) > 0 {
		scopeOr = append(scopeOr, bson.D{
			{Key: "scope_type", Value: models.ScopeService},
			{Key: "scope_value", Value: bson.D{{Key: "$in", Value: serviceValues}}},
		})
	}
	if len(pageValues) > 0 {
		scopeOr = append(scopeOr, bson.D{
			{Key: "scope_type", Value: models.ScopePage},
			{Key: "scope_value", Value: bson.D{{Key: "$in", Value: pageValues}}},
		})
	}

	filter := bson.D{
		{Key: "$and", Value: bson.A{activeFilter, bson.D{{Key: "$or", Value: scopeOr}}}},
	}

	_, span := otel.Tracer("banners").Start(c.Request.Context(), "list-banners",
		oteltrace.WithAttributes(attribute.StringSlice("scope", scopes)))
	banners, err := database.Query[models.BannerMessage](constants.BannerCollection, filter, nil, nil, 0, 1000)
	span.End()
	if err != nil {
		logging.Logger.Error("Failed to query banners", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to list banners"})
		return
	}

	sort.SliceStable(banners, func(i, j int) bool {
		return banners[i].Severity.Rank() < banners[j].Severity.Rank()
	})

	if banners == nil {
		banners = []*models.BannerMessage{}
	}
	c.JSON(http.StatusOK, banners)
}

// Update a banner message.
//
//	@Summary		Update a banner message
//	@Description	Replaces the mutable fields of an existing banner message. expires_at is required.
//	@Tags			banners
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Banner ID"
//	@Param			banner	body		updateBannerRequest		true	"Updated banner message"
//	@Success		200		{object}	models.BannerMessage
//	@Failure		400		{object}	apiv.ErrorVO
//	@Failure		404		{object}	apiv.ErrorVO
//	@Failure		500		{object}	apiv.ErrorVO
//	@Router			/banners/{id} [put]
func updateBanner(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_id", Message: "id must be a valid object id"})
		return
	}

	var req updateBannerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	existing, err := database.Get[models.BannerMessage](constants.BannerCollection, id)
	if err != nil {
		logging.Logger.Error("Failed to look up banner", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "lookup_failed", Message: "failed to look up banner"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "banner not found"})
		return
	}

	updated := &models.BannerMessage{
		ID:         id,
		ScopeType:  req.ScopeType,
		ScopeValue: req.ScopeValue,
		Severity:   req.Severity,
		Message:    req.Message,
		StartsAt:   req.StartsAt,
		ExpiresAt:  req.ExpiresAt,
		CreatedBy:  existing.CreatedBy,
		CreatedAt:  existing.CreatedAt,
		UpdatedAt:  time.Now().UTC(),
	}

	if err := updated.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "validation_failed", Message: err.Error()})
		return
	}

	_, span := otel.Tracer("banners").Start(c.Request.Context(), "update-banner",
		oteltrace.WithAttributes(attribute.String("id", id.Hex())))
	matched, _, err := database.Update(constants.BannerCollection, id, updated)
	span.End()
	if err != nil {
		logging.Logger.Error("Failed to update banner", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "update_failed", Message: "failed to update banner"})
		return
	}
	if matched == 0 {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "banner not found"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

// Delete a banner message.
//
//	@Summary		Delete a banner message
//	@Description	Deletes a banner message; it is immediately excluded from subsequent queries.
//	@Tags			banners
//	@Param			id	path	string	true	"Banner ID"
//	@Success		204	{object}	interface{}
//	@Failure		400	{object}	apiv.ErrorVO
//	@Failure		404	{object}	apiv.ErrorVO
//	@Failure		500	{object}	apiv.ErrorVO
//	@Router			/banners/{id} [delete]
func deleteBanner(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_id", Message: "id must be a valid object id"})
		return
	}

	_, span := otel.Tracer("banners").Start(c.Request.Context(), "delete-banner",
		oteltrace.WithAttributes(attribute.String("id", id.Hex())))
	deleted, err := database.Delete[models.BannerMessage](constants.BannerCollection, id)
	span.End()
	if err != nil {
		logging.Logger.Error("Failed to delete banner", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "delete_failed", Message: "failed to delete banner"})
		return
	}
	if !deleted {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "banner not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// parseScope splits a "type" or "type:value" scope query param into its parts.
// An unrecognized type is passed through as-is and matched against nothing, since the
// filter builder only special-cases service/page.
func parseScope(raw string) (models.ScopeType, string) {
	parts := strings.SplitN(raw, ":", 2)
	scopeType := models.ScopeType(parts[0])
	if len(parts) == 2 {
		return scopeType, parts[1]
	}
	return scopeType, ""
}
