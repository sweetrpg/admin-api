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

func setupMaintenanceModeHandlers(g *gin.Engine, authzClient *authz.Client) {
	logging.Logger.Info("Setting up maintenance-mode endpoint handlers...")

	g.GET("/maintenance-modes", listMaintenanceModes)
	g.GET("/maintenance-modes/active", listActiveMaintenanceModes)

	writes := g.Group("/maintenance-modes", middleware.WriteAuth(authzClient))
	writes.POST("", createMaintenanceMode)
	writes.PUT("/:id", updateMaintenanceMode)
	writes.DELETE("/:id", deleteMaintenanceMode)
}

// maintenanceModeRequest is the payload accepted by POST /maintenance-modes and
// PUT /maintenance-modes/:id.
type maintenanceModeRequest struct {
	ScopeType   models.MaintenanceScopeType `json:"scope_type" binding:"required"`
	ScopeValue  string                      `json:"scope_value"`
	Enabled     bool                        `json:"enabled"`
	StartsAt    time.Time                   `json:"starts_at" binding:"required"`
	EndsAt      *time.Time                  `json:"ends_at"`
	Label       string                      `json:"label" binding:"required"`
	Description string                      `json:"description" binding:"required"`
}

// Create or update a maintenance-mode record for a scope.
//
//	@Summary		Create or update a maintenance-mode record
//	@Description	Creates a maintenance-mode record for the given scope, or updates the existing record for that scope in place if one already exists - at most one record per scope is kept. Requires a forwarded user bearer token carrying the admin role.
//	@Tags			maintenance-modes
//	@Accept			json
//	@Produce		json
//	@Param			Authorization				header					string	true	"Bearer user access token"
//	@Param			maintenance_mode			body		maintenanceModeRequest	true	"Maintenance-mode record"
//	@Success		200							{object}	models.MaintenanceMode
//	@Success		201							{object}	models.MaintenanceMode
//	@Failure		400							{object}	apiv.ErrorVO
//	@Failure		401							{object}	apiv.ErrorVO
//	@Failure		500							{object}	apiv.ErrorVO
//	@Router			/maintenance-modes [post]
func createMaintenanceMode(c *gin.Context) {
	var req maintenanceModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	now := time.Now().UTC()
	sub := c.GetString(middleware.ActingUserSubKey)
	record := &models.MaintenanceMode{
		ScopeType:   req.ScopeType,
		ScopeValue:  req.ScopeValue,
		Enabled:     req.Enabled,
		StartsAt:    req.StartsAt,
		EndsAt:      req.EndsAt,
		Label:       req.Label,
		Description: req.Description,
		CreatedBy:   sub,
		CreatedAt:   now,
		UpdatedBy:   sub,
		UpdatedAt:   now,
	}

	if err := record.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "validation_failed", Message: err.Error()})
		return
	}

	auditID, err := models.RecordAuditAttempt(c.GetString(middleware.ActingUserSubKey), "upsert_maintenance_mode", "", record.Label)
	if err != nil {
		logging.Logger.Error("Failed to record audit attempt", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "audit_failed", Message: "failed to save maintenance mode"})
		return
	}

	_, span := otel.Tracer("maintenance-modes").Start(c.Request.Context(), "upsert-maintenance-mode",
		oteltrace.WithAttributes(
			attribute.String("scope_type", string(record.ScopeType)),
			attribute.String("scope_value", record.ScopeValue)))
	defer span.End()

	existing, err := findMaintenanceModeByScope(record.ScopeType, record.ScopeValue)
	if err != nil {
		logging.Logger.Error("Failed to look up existing maintenance mode", "error", err.Error())
		completeAudit(auditID, models.AuditFailed, err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "lookup_failed", Message: "failed to save maintenance mode"})
		return
	}

	if existing != nil {
		record.ID = existing.ID
		record.CreatedAt = existing.CreatedAt
		record.CreatedBy = existing.CreatedBy
		if _, _, err := database.Update(constants.MaintenanceModeCollection, record.ID, record); err != nil {
			logging.Logger.Error("Failed to update maintenance mode", "error", err.Error())
			completeAudit(auditID, models.AuditFailed, err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "update_failed", Message: "failed to save maintenance mode"})
			return
		}
		completeAudit(auditID, models.AuditSucceeded, "")
		c.JSON(http.StatusOK, record)
		return
	}

	id, err := database.Insert(constants.MaintenanceModeCollection, record)
	if err != nil {
		logging.Logger.Error("Failed to insert maintenance mode", "error", err.Error())
		completeAudit(auditID, models.AuditFailed, err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "insert_failed", Message: "failed to save maintenance mode"})
		return
	}
	record.ID = id
	completeAudit(auditID, models.AuditSucceeded, "")

	c.JSON(http.StatusCreated, record)
}

// findMaintenanceModeByScope returns the existing record for a scope, or nil if none
// exists.
func findMaintenanceModeByScope(scopeType models.MaintenanceScopeType, scopeValue string) (*models.MaintenanceMode, error) {
	filter := bson.D{
		{Key: "scope_type", Value: scopeType},
		{Key: "scope_value", Value: scopeValue},
	}
	records, err := database.Query[models.MaintenanceMode](constants.MaintenanceModeCollection, filter, nil, nil, 0, 1)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return records[0], nil
}

// List every maintenance-mode record, for the admin management view.
//
//	@Summary		List maintenance-mode records
//	@Description	Returns every maintenance-mode record (platform and every known service scope) regardless of enabled state, for admin-web's management view.
//	@Tags			maintenance-modes
//	@Produce		json
//	@Success		200	{array}		models.MaintenanceMode
//	@Failure		500	{object}	apiv.ErrorVO
//	@Router			/maintenance-modes [get]
func listMaintenanceModes(c *gin.Context) {
	_, span := otel.Tracer("maintenance-modes").Start(c.Request.Context(), "list-maintenance-modes")
	records, err := database.Query[models.MaintenanceMode](constants.MaintenanceModeCollection, bson.D{}, nil, nil, 0, 1000)
	span.End()
	if err != nil {
		logging.Logger.Error("Failed to query maintenance modes", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to list maintenance modes"})
		return
	}

	if records == nil {
		records = []*models.MaintenanceMode{}
	}
	c.JSON(http.StatusOK, records)
}

// List active (enabled) maintenance-mode records matching the requested scopes, for
// consuming frontends.
//
//	@Summary		List active maintenance-mode records for a set of scopes
//	@Description	Returns enabled maintenance-mode records matching any of the requested scopes; platform-scoped records are always included. scope values look like "platform" or "service:catalog".
//	@Tags			maintenance-modes
//	@Produce		json
//	@Param			scope	query		[]string	true	"Repeated scope filter, e.g. scope=platform&scope=service:catalog"
//	@Success		200		{array}		models.MaintenanceMode
//	@Failure		400		{object}	apiv.ErrorVO
//	@Failure		500		{object}	apiv.ErrorVO
//	@Router			/maintenance-modes/active [get]
func listActiveMaintenanceModes(c *gin.Context) {
	scopes := c.QueryArray("scope")
	if len(scopes) == 0 {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "missing_scope", Message: "at least one scope query param is required"})
		return
	}

	filter := buildActiveMaintenanceModeFilter(scopes)

	_, span := otel.Tracer("maintenance-modes").Start(c.Request.Context(), "list-active-maintenance-modes",
		oteltrace.WithAttributes(attribute.StringSlice("scope", scopes)))
	records, err := database.Query[models.MaintenanceMode](constants.MaintenanceModeCollection, filter, nil, nil, 0, 1000)
	span.End()
	if err != nil {
		logging.Logger.Error("Failed to query active maintenance modes", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to list active maintenance modes"})
		return
	}

	if records == nil {
		records = []*models.MaintenanceMode{}
	}
	c.JSON(http.StatusOK, records)
}

// Update a maintenance-mode record by id.
//
//	@Summary		Update a maintenance-mode record
//	@Description	Replaces the mutable fields of an existing maintenance-mode record. Requires a forwarded user bearer token carrying the admin role.
//	@Tags			maintenance-modes
//	@Accept			json
//	@Produce		json
//	@Param			Authorization				header					string	true	"Bearer user access token"
//	@Param			id							path		string					true	"Maintenance-mode record ID"
//	@Param			maintenance_mode			body		maintenanceModeRequest	true	"Updated maintenance-mode record"
//	@Success		200							{object}	models.MaintenanceMode
//	@Failure		400							{object}	apiv.ErrorVO
//	@Failure		401							{object}	apiv.ErrorVO
//	@Failure		404							{object}	apiv.ErrorVO
//	@Failure		500							{object}	apiv.ErrorVO
//	@Router			/maintenance-modes/{id} [put]
func updateMaintenanceMode(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_id", Message: "id must be a valid object id"})
		return
	}

	var req maintenanceModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	existing, err := database.Get[models.MaintenanceMode](constants.MaintenanceModeCollection, id)
	if err != nil {
		logging.Logger.Error("Failed to look up maintenance mode", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "lookup_failed", Message: "failed to look up maintenance mode"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "maintenance mode not found"})
		return
	}

	updated := &models.MaintenanceMode{
		ID:          id,
		ScopeType:   req.ScopeType,
		ScopeValue:  req.ScopeValue,
		Enabled:     req.Enabled,
		StartsAt:    req.StartsAt,
		EndsAt:      req.EndsAt,
		Label:       req.Label,
		Description: req.Description,
		CreatedBy:   existing.CreatedBy,
		CreatedAt:   existing.CreatedAt,
		UpdatedBy:   c.GetString(middleware.ActingUserSubKey),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := updated.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "validation_failed", Message: err.Error()})
		return
	}

	auditID, err := models.RecordAuditAttempt(c.GetString(middleware.ActingUserSubKey), "update_maintenance_mode", id.Hex(), updated.Label)
	if err != nil {
		logging.Logger.Error("Failed to record audit attempt", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "audit_failed", Message: "failed to update maintenance mode"})
		return
	}

	_, span := otel.Tracer("maintenance-modes").Start(c.Request.Context(), "update-maintenance-mode",
		oteltrace.WithAttributes(attribute.String("id", id.Hex())))
	matched, _, err := database.Update(constants.MaintenanceModeCollection, id, updated)
	span.End()
	if err != nil {
		logging.Logger.Error("Failed to update maintenance mode", "error", err.Error())
		completeAudit(auditID, models.AuditFailed, err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "update_failed", Message: "failed to update maintenance mode"})
		return
	}
	if matched == 0 {
		completeAudit(auditID, models.AuditFailed, "maintenance mode not found")
		c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "maintenance mode not found"})
		return
	}
	completeAudit(auditID, models.AuditSucceeded, "")

	c.JSON(http.StatusOK, updated)
}

// Delete a maintenance-mode record.
//
//	@Summary		Delete a maintenance-mode record
//	@Description	Deletes a maintenance-mode record; it is immediately excluded from subsequent queries. Requires a forwarded user bearer token carrying the admin role.
//	@Tags			maintenance-modes
//	@Param			Authorization				header	string	true	"Bearer user access token"
//	@Param			id							path	string	true	"Maintenance-mode record ID"
//	@Success		204							{object}	interface{}
//	@Failure		400							{object}	apiv.ErrorVO
//	@Failure		401							{object}	apiv.ErrorVO
//	@Failure		404							{object}	apiv.ErrorVO
//	@Failure		500							{object}	apiv.ErrorVO
//	@Router			/maintenance-modes/{id} [delete]
func deleteMaintenanceMode(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_id", Message: "id must be a valid object id"})
		return
	}

	auditID, err := models.RecordAuditAttempt(c.GetString(middleware.ActingUserSubKey), "delete_maintenance_mode", id.Hex(), "")
	if err != nil {
		logging.Logger.Error("Failed to record audit attempt", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "audit_failed", Message: "failed to delete maintenance mode"})
		return
	}

	_, span := otel.Tracer("maintenance-modes").Start(c.Request.Context(), "delete-maintenance-mode",
		oteltrace.WithAttributes(attribute.String("id", id.Hex())))
	deleted, err := database.Delete[models.MaintenanceMode](constants.MaintenanceModeCollection, id)
	span.End()
	if err != nil {
		logging.Logger.Error("Failed to delete maintenance mode", "error", err.Error())
		completeAudit(auditID, models.AuditFailed, err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "delete_failed", Message: "failed to delete maintenance mode"})
		return
	}
	if !deleted {
		completeAudit(auditID, models.AuditFailed, "maintenance mode not found")
		c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "maintenance mode not found"})
		return
	}
	completeAudit(auditID, models.AuditSucceeded, "")

	c.Status(http.StatusNoContent)
}

// extractServiceScopeValues returns the service names from a list of "service:<name>"
// scope query params, ignoring "platform" and any other scope type.
func extractServiceScopeValues(scopes []string) []string {
	var serviceValues []string
	for _, raw := range scopes {
		scopeType, scopeValue := parseMaintenanceScope(raw)
		if scopeType == models.MaintenanceScopeService && scopeValue != "" {
			serviceValues = append(serviceValues, scopeValue)
		}
	}
	return serviceValues
}

// buildActiveMaintenanceModeFilter builds the Mongo filter for enabled records matching any of
// the requested scopes, whose window actually contains now - `enabled` alone is not enough,
// since it doesn't auto-clear once `ends_at` passes (see models.MaintenanceMode).
func buildActiveMaintenanceModeFilter(scopes []string) bson.D {
	serviceValues := extractServiceScopeValues(scopes)

	scopeOr := bson.A{bson.D{{Key: "scope_type", Value: models.MaintenanceScopePlatform}}}
	if len(serviceValues) > 0 {
		scopeOr = append(scopeOr, bson.D{
			{Key: "scope_type", Value: models.MaintenanceScopeService},
			{Key: "scope_value", Value: bson.D{{Key: "$in", Value: serviceValues}}},
		})
	}

	now := time.Now()
	endsOr := bson.A{
		bson.D{{Key: "ends_at", Value: nil}},
		bson.D{{Key: "ends_at", Value: bson.D{{Key: "$gt", Value: now}}}},
	}

	return bson.D{
		{Key: "enabled", Value: true},
		{Key: "starts_at", Value: bson.D{{Key: "$lte", Value: now}}},
		{Key: "$or", Value: scopeOr},
		{Key: "$and", Value: bson.A{bson.D{{Key: "$or", Value: endsOr}}}},
	}
}

// parseMaintenanceScope splits a "type" or "type:value" scope query param into its
// parts. An unrecognized type is passed through as-is and matched against nothing,
// since the filter builder only special-cases service.
func parseMaintenanceScope(raw string) (models.MaintenanceScopeType, string) {
	parts := strings.SplitN(raw, ":", 2)
	scopeType := models.MaintenanceScopeType(parts[0])
	if len(parts) == 2 {
		return scopeType, parts[1]
	}
	return scopeType, ""
}
