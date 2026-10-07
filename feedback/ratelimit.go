package feedback

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
	apicoreconstants "github.com/sweetrpg/api-core.go/constants"
	"github.com/sweetrpg/api-core.go/ratelimit"
	"github.com/sweetrpg/api-core.go/util"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/admin-api/constants"
	"github.com/sweetrpg/common.go/logging"
)

// tierFeedback is this route's own rate-limit tier name, kept separate from
// api-core.go/ratelimit's cheap/standard tiers so it doesn't change the budget for any other
// admin-api route (see design.md's "dedicated feedback-specific rate-limit tier" decision).
const tierFeedback = "feedback"

// RateLimitMiddleware enforces a per-client budget on just the route it's mounted on, stricter
// than and layered on top of admin-api's existing service-wide limiter. Fails closed (503) if
// pool is nil or Redis is unreachable, matching api-core.go/ratelimit.Middleware's behavior.
func RateLimitMiddleware(pool *redis.Pool) gin.HandlerFunc {
	limiter := ratelimit.New(pool, map[string]ratelimit.Tier{
		tierFeedback: {
			Limit:  util.GetEnvInt(constants.RATE_LIMIT_FEEDBACK, constants.RateLimitFeedbackDefault),
			Window: util.GetEnvInt(constants.RATE_LIMIT_FEEDBACK_WINDOW_SECONDS, constants.RateLimitFeedbackWindowDefault),
		},
	})

	return func(c *gin.Context) {
		if pool == nil {
			logging.Logger.Error("Feedback rate-limit store not configured; rejecting request (fail closed)")
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, apiv.ErrorVO{
				Error:   apicoreconstants.ErrorRateLimitUnavailable,
				Message: "Rate limiting is temporarily unavailable",
			})
			return
		}

		clientKey := ratelimit.ClientKey(c)
		allowed, err := limiter.Allow(c.Request.Context(), clientKey, tierFeedback)
		if err != nil {
			logging.Logger.Error("Feedback rate-limit backend unreachable; rejecting request (fail closed)", "error", err.Error())
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, apiv.ErrorVO{
				Error:   apicoreconstants.ErrorRateLimitUnavailable,
				Message: "Rate limiting is temporarily unavailable",
			})
			return
		}
		if !allowed {
			logging.Logger.Warn("Feedback rate limit exceeded", "client", clientKey)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, apiv.ErrorVO{
				Error:   apicoreconstants.ErrorRateLimited,
				Message: "Limit exceeded",
			})
			return
		}
		c.Next()
	}
}
