package constants

// Environment variable names
const (
	HEALTH_TOKEN             = "HEALTH_TOKEN"
	ALLOWED_ORIGINS          = "ALLOWED_ORIGINS"
	PYROSCOPE_SERVER_ADDRESS = "PYROSCOPE_SERVER_ADDRESS"
	PYROSCOPE_TENANT_ID      = "PYROSCOPE_TENANT_ID"

	// INTERNAL_SERVICE_TOKEN gates write routes; see server/middleware.WriteAuth.
	INTERNAL_SERVICE_TOKEN = "INTERNAL_SERVICE_TOKEN"
)

// Value constants
const (
	ServiceName = "admin-api"

	// ProfilingEnabledFlag is the feature-flag key gating continuous
	// profiling, evaluated via api-core.go/featureflags. Replaces the old
	// PYROSCOPE_SERVER_ADDRESS-presence check; see
	// openspec/changes/pyroscope-profiling-feature-flag in sweetrpg/platform.
	ProfilingEnabledFlag = "profiling-enabled"

	// BannerCollection is the MongoDB collection name for banner messages.
	BannerCollection = "banners"

	// MaintenanceModeCollection is the MongoDB collection name for maintenance-mode
	// records.
	MaintenanceModeCollection = "maintenance_modes"

	// AdminActionAuditLogCollection is the MongoDB collection name for write-route
	// audit records.
	AdminActionAuditLogCollection = "admin_action_audit_logs"
)
