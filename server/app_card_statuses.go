package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/admin-api/authz"
	"github.com/sweetrpg/admin-api/constants"
	"github.com/sweetrpg/admin-api/models"
	"github.com/sweetrpg/admin-api/server/middleware"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/mongodb.go/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func setupAppCardStatusHandlers(g *gin.Engine, authzClient *authz.Client) {
	logging.Logger.Info("Setting up app-card-status endpoint handlers...")

	g.GET("/app-card-statuses", listAppCardStatuses)
	g.GET("/app-card-statuses/active", listActiveAppCardStatuses)

	writes := g.Group("/app-card-statuses", middleware.WriteAuth(authzClient))
	writes.POST("", createAppCardStatus)
	writes.PUT("/:id", updateAppCardStatus)
	writes.DELETE("/:id", deleteAppCardStatus)
}

// appCardStatusRequest is the payload accepted by POST /app-card-statuses and
// PUT /app-card-statuses/:id.
type appCardStatusRequest struct {
	ScopeValue string `json:"scope_value" binding:"required"`
	Enabled    bool   `json:"enabled"`
	Label      string `json:"label" binding:"required"`
}

// Create or update an app-card-status record for a service scope.
//
//	@Summary		Create or update an app-card-status record
//	@Description	Creates an app-card-status record for the given service scope, or updates the existing record for that scope in place if one already exists - at most one record per scope is kept. Requires a forwarded user bearer token carrying the admin role.
//	@Tags			app-card-statuses
//	@Accept			json
//	@Produce		json
//	@Param			Authorization			header					string	true	"Bearer user access token"
//	@Param			app_card_status			body		appCardStatusRequest	true	"App card status record"
//	@Success		200						{object}	models.AppCardStatus
//	@Success		201						{object}	models.AppCardStatus
//	@Failure		400						{object}	apiv.ErrorVO
//	@Failure		401						{object}	apiv.ErrorVO
//	@Failure		500						{object}	apiv.ErrorVO
//	@Router			/app-card-statuses [post]
func createAppCardStatus(c *gin.Context) {
	var req appCardStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	now := time.Now().UTC()
	record := &models.AppCardStatus{
		ScopeType:  models.AppCardStatusScopeService,
		ScopeValue: req.ScopeValue,
		Enabled:    req.Enabled,
		Label:      req.Label,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := record.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "validation_failed", Message: err.Error()})
		return
	}

	auditID, err := models.RecordAuditAttempt(c.GetString(middleware.ActingUserSubKey), "upsert_app_card_status", "", record.Label)
	if err != nil {
		logging.Logger.Error("Failed to record audit attempt", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "audit_failed", Message: "failed to save app card status"})
		return
	}

	_, span := otel.Tracer("app-card-statuses").Start(c.Request.Context(), "upsert-app-card-status",
		oteltrace.WithAttributes(attribute.String("scope_value", record.ScopeValue)))
	defer span.End()

	existing, err := findAppCardStatusByScope(record.ScopeValue)
	if err != nil {
		logging.Logger.Error("Failed to look up existing app card status", "error", err.Error())
		completeAudit(auditID, models.AuditFailed, err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "lookup_failed", Message: "failed to save app card status"})
		return
	}

	if existing != nil {
		record.ID = existing.ID
		record.CreatedAt = existing.CreatedAt
		if _, _, err := database.Update(constants.AppCardStatusCollection, record.ID, record); err != nil {
			logging.Logger.Error("Failed to update app card status", "error", err.Error())
			completeAudit(auditID, models.AuditFailed, err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "update_failed", Message: "failed to save app card status"})
			return
		}
		completeAudit(auditID, models.AuditSucceeded, "")
		c.JSON(http.StatusOK, record)
		return
	}

	id, err := database.Insert(constants.AppCardStatusCollection, record)
	if err != nil {
		logging.Logger.Error("Failed to insert app card status", "error", err.Error())
		completeAudit(auditID, models.AuditFailed, err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "insert_failed", Message: "failed to save app card status"})
		return
	}
	record.ID = id
	completeAudit(auditID, models.AuditSucceeded, "")

	c.JSON(http.StatusCreated, record)
}

// findAppCardStatusByScope returns the existing record for a service scope, or nil if none exists.
func findAppCardStatusByScope(scopeValue string) (*models.AppCardStatus, error) {
	filter := bson.D{
		{Key: "scope_type", Value: models.AppCardStatusScopeService},
		{Key: "scope_value", Value: scopeValue},
	}
	records, err := database.Query[models.AppCardStatus](constants.AppCardStatusCollection, filter, nil, nil, 0, 1)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return records[0], nil
}

// List every app-card-status record, for the admin management view.
//
//	@Summary		List app-card-status records
//	@Description	Returns every app-card-status record regardless of enabled state, for admin-web's management view.
//	@Tags			app-card-statuses
//	@Produce		json
//	@Success		200	{array}		models.AppCardStatus
//	@Failure		500	{object}	apiv.ErrorVO
//	@Router			/app-card-statuses [get]
func listAppCardStatuses(c *gin.Context) {
	_, span := otel.Tracer("app-card-statuses").Start(c.Request.Context(), "list-app-card-statuses")
	records, err := database.Query[models.AppCardStatus](constants.AppCardStatusCollection, bson.D{}, nil, nil, 0, 1000)
	span.End()
	if err != nil {
		logging.Logger.Error("Failed to query app card statuses", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to list app card statuses"})
		return
	}

	if records == nil {
		records = []*models.AppCardStatus{}
	}
	c.JSON(http.StatusOK, records)
}

// List active (enabled) app-card-status records matching the requested service scopes.
//
//	@Summary		List active app-card-status records for a set of service scopes
//	@Description	Returns enabled app-card-status records matching any of the requested service scopes. scope values look like "service:catalog".
//	@Tags			app-card-statuses
//	@Produce		json
//	@Param			scope	query		[]string	true	"Repeated scope filter, e.g. scope=service:catalog&scope=service:shelf"
//	@Success		200		{array}		models.AppCardStatus
//	@Failure		400		{object}	apiv.ErrorVO
//	@Failure		500		{object}	apiv.ErrorVO
//	@Router			/app-card-statuses/active [get]
func listActiveAppCardStatuses(c *gin.Context) {
	scopes := c.QueryArray("scope")
	if len(scopes) == 0 {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "missing_scope", Message: "at least one scope query param is required"})
		return
	}

	serviceValues := extractAppCardServiceScopeValues(scopes)
	if len(serviceValues) == 0 {
		c.JSON(http.StatusOK, []*models.AppCardStatus{})
		return
	}

	_, span := otel.Tracer("app-card-statuses").Start(c.Request.Context(), "list-active-app-card-statuses",
		oteltrace.WithAttributes(attribute.StringSlice("scope", scopes)))

	filter := bson.D{
		{Key: "enabled", Value: true},
		{Key: "scope_type", Value: models.AppCardStatusScopeService},
		{Key: "scope_value", Value: bson.D{{Key: "$in", Value: serviceValues}}},
	}

	records, err := database.Query[models.AppCardStatus](constants.AppCardStatusCollection, filter, nil, nil, 0, 1000)
	span.End()
	if err != nil {
		logging.Logger.Error("Failed to query active app card statuses", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to list active app card statuses"})
		return
	}

	if records == nil {
		records = []*models.AppCardStatus{}
	}
	c.JSON(http.StatusOK, records)
}

// Update an app-card-status record by id.
//
//	@Summary		Update an app-card-status record
//	@Description	Replaces the mutable fields of an existing app-card-status record. Requires a forwarded user bearer token carrying the admin role.
//	@Tags			app-card-statuses
//	@Accept			json
//	@Produce		json
//	@Param			Authorization			header					string	true	"Bearer user access token"
//	@Param			id						path		string					true	"App card status record ID"
//	@Param			app_card_status			body		appCardStatusRequest	true	"Updated app card status record"
//	@Success		200						{object}	models.AppCardStatus
//	@Failure		400						{object}	apiv.ErrorVO
//	@Failure		401						{object}	apiv.ErrorVO
//	@Failure		404						{object}	apiv.ErrorVO
//	@Failure		500						{object}	apiv.ErrorVO
//	@Router			/app-card-statuses/{id} [put]
func updateAppCardStatus(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_id", Message: "id must be a valid object id"})
		return
	}

	var req appCardStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	existing, err := database.Get[models.AppCardStatus](constants.AppCardStatusCollection, id)
	if err != nil {
		logging.Logger.Error("Failed to look up app card status", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "lookup_failed", Message: "failed to look up app card status"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "app card status not found"})
		return
	}

	updated := &models.AppCardStatus{
		ID:         id,
		ScopeType:  models.AppCardStatusScopeService,
		ScopeValue: req.ScopeValue,
		Enabled:    req.Enabled,
		Label:      req.Label,
		CreatedAt:  existing.CreatedAt,
		UpdatedAt:  time.Now().UTC(),
	}

	if err := updated.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "validation_failed", Message: err.Error()})
		return
	}

	auditID, err := models.RecordAuditAttempt(c.GetString(middleware.ActingUserSubKey), "update_app_card_status", id.Hex(), updated.Label)
	if err != nil {
		logging.Logger.Error("Failed to record audit attempt", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "audit_failed", Message: "failed to update app card status"})
		return
	}

	_, span := otel.Tracer("app-card-statuses").Start(c.Request.Context(), "update-app-card-status",
		oteltrace.WithAttributes(attribute.String("id", id.Hex())))
	matched, _, err := database.Update(constants.AppCardStatusCollection, id, updated)
	span.End()
	if err != nil {
		logging.Logger.Error("Failed to update app card status", "error", err.Error())
		completeAudit(auditID, models.AuditFailed, err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "update_failed", Message: "failed to update app card status"})
		return
	}
	if matched == 0 {
		completeAudit(auditID, models.AuditFailed, "app card status not found")
		c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "app card status not found"})
		return
	}
	completeAudit(auditID, models.AuditSucceeded, "")

	c.JSON(http.StatusOK, updated)
}

// Delete an app-card-status record.
//
//	@Summary		Delete an app-card-status record
//	@Description	Deletes an app-card-status record; it is immediately excluded from subsequent queries. Requires a forwarded user bearer token carrying the admin role.
//	@Tags			app-card-statuses
//	@Param			Authorization	header	string	true	"Bearer user access token"
//	@Param			id				path	string	true	"App card status record ID"
//	@Success		204				{object}	interface{}
//	@Failure		400				{object}	apiv.ErrorVO
//	@Failure		401				{object}	apiv.ErrorVO
//	@Failure		404				{object}	apiv.ErrorVO
//	@Failure		500				{object}	apiv.ErrorVO
//	@Router			/app-card-statuses/{id} [delete]
func deleteAppCardStatus(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_id", Message: "id must be a valid object id"})
		return
	}

	auditID, err := models.RecordAuditAttempt(c.GetString(middleware.ActingUserSubKey), "delete_app_card_status", id.Hex(), "")
	if err != nil {
		logging.Logger.Error("Failed to record audit attempt", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "audit_failed", Message: "failed to delete app card status"})
		return
	}

	_, span := otel.Tracer("app-card-statuses").Start(c.Request.Context(), "delete-app-card-status",
		oteltrace.WithAttributes(attribute.String("id", id.Hex())))
	deleted, err := database.Delete[models.AppCardStatus](constants.AppCardStatusCollection, id)
	span.End()
	if err != nil {
		logging.Logger.Error("Failed to delete app card status", "error", err.Error())
		completeAudit(auditID, models.AuditFailed, err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "delete_failed", Message: "failed to delete app card status"})
		return
	}
	if !deleted {
		completeAudit(auditID, models.AuditFailed, "app card status not found")
		c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "app card status not found"})
		return
	}
	completeAudit(auditID, models.AuditSucceeded, "")

	c.Status(http.StatusNoContent)
}

// extractAppCardServiceScopeValues returns the service names from a list of "service:<name>"
// scope query params, ignoring any other scope type.
func extractAppCardServiceScopeValues(scopes []string) []string {
	var serviceValues []string
	for _, raw := range scopes {
		scopeType, scopeValue := parseAppCardScope(raw)
		if scopeType == "service" && scopeValue != "" {
			serviceValues = append(serviceValues, scopeValue)
		}
	}
	return serviceValues
}

// parseAppCardScope splits a "type" or "type:value" scope query param into its parts.
func parseAppCardScope(raw string) (string, string) {
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return parts[0], ""
}
