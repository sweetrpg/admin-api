package constants

import "time"

// Environment variable names
const (
	HEALTH_TOKEN             = "HEALTH_TOKEN"
	ALLOWED_ORIGINS          = "ALLOWED_ORIGINS"
	PYROSCOPE_SERVER_ADDRESS = "PYROSCOPE_SERVER_ADDRESS"
	PYROSCOPE_TENANT_ID      = "PYROSCOPE_TENANT_ID"

	// AUTH_API_URL is auth-api's base URL, used by server/middleware.WriteAuth to verify
	// forwarded user bearer tokens via /authz/check.
	AUTH_API_URL = "AUTH_API_URL"

	// USERS_API_URL points at users-api's base URL (e.g.
	// http://api-v1.sweetrpg-users.svc.cluster.local:8000), used to resolve the verified
	// subject to its canonical users._id for write-path created_by/updated_by stamps. See
	// canonical-user-ids-across-services in sweetrpg/platform.
	USERS_API_URL = "USERS_API_URL"

	// GITHUB_FEEDBACK_TOKEN authenticates the feedback package's GitHub Issues API calls.
	// Scoped to Issues:write on sweetrpg/platform only - see design.md.
	GITHUB_FEEDBACK_TOKEN = "GITHUB_FEEDBACK_TOKEN"

	// RATE_LIMIT_FEEDBACK and RATE_LIMIT_FEEDBACK_WINDOW_SECONDS tune the feedback-specific
	// rate-limit tier, stricter than api-core.go/ratelimit's standard tier. See
	// feedback.RateLimitMiddleware.
	RATE_LIMIT_FEEDBACK                = "RATE_LIMIT_FEEDBACK"
	RATE_LIMIT_FEEDBACK_WINDOW_SECONDS = "RATE_LIMIT_FEEDBACK_WINDOW_SECONDS"
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

	// AppCardStatusCollection is the MongoDB collection name for app-card-status
	// records.
	AppCardStatusCollection = "app_card_statuses"

	// AdminActionAuditLogCollection is the MongoDB collection name for write-route
	// audit records.
	AdminActionAuditLogCollection = "admin_action_audit_logs"

	// RateLimitFeedbackDefault and RateLimitFeedbackWindowDefault are the feedback tier's
	// fallback budget when RATE_LIMIT_FEEDBACK[_WINDOW_SECONDS] aren't set: 5 submissions
	// per 5 minutes per client, well under the platform-default standard tier (see
	// design.md's "dedicated feedback-specific rate-limit tier" decision).
	RateLimitFeedbackDefault       = 5
	RateLimitFeedbackWindowDefault = 300

	// FeedbackTitleMaxLength and FeedbackBodyMaxLength cap submitted text, rejected with a
	// 400 when exceeded (see the anonymous-feedback-submission spec's "Oversized submission
	// rejected" scenario).
	FeedbackTitleMaxLength = 200
	FeedbackBodyMaxLength  = 10000

	// FeedbackMinFillTime is the minimum time between the form rendering (client-reported
	// rendered_at) and submission for a human to plausibly have filled it out. Submissions
	// faster than this are silently discarded - see design.md's honeypot/fill-time decision.
	FeedbackMinFillTime = 3 * time.Second

	// FeedbackSourceLabel is applied to every issue created from a feedback submission, so a
	// spam wave can be bulk-filtered/closed without affecting other issues.
	FeedbackSourceLabel = "source:in-app-feedback"
)
