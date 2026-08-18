// Package middleware provides Gin middleware for admin-api's route groups.
package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/admin-api/authz"
	"github.com/sweetrpg/admin-api/constants"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/common.go/util"
)

const (
	internalServiceTokenHeader = "X-Internal-Service-Token"
	actingUserSubHeader        = "X-Acting-User-Sub"
	bearerPrefix               = "Bearer "

	// ActingUserSubKey is the gin.Context key write handlers read to get the
	// attributed acting user after WriteAuth has validated the request.
	ActingUserSubKey = "actingUserSub"
)

// WriteAuth requires either a forwarded user bearer token carrying the admin role (verified
// against auth-api's /authz/check) or, as a legacy fallback during migration, a valid
// X-Internal-Service-Token header (compared constant-time against INTERNAL_SERVICE_TOKEN). The
// legacy fallback authorizes the request but has no verified user to attribute it to; callers
// using it must still send a value acting-user identity out of band until they migrate.
// On success, the acting user (from auth-api's verified token subject) is stashed on the
// context under ActingUserSubKey for handlers to attribute their audit records to.
func WriteAuth(client *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token := bearerToken(c); token != "" {
			result, err := client.Check(c.Request.Context(), token, constants.ServiceName)
			if err != nil {
				if _, ok := err.(authz.InvalidTokenError); ok {
					unauthorized(c)
					return
				}
				logging.Logger.Error("authz check failed", "error", err.Error())
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, apiv.ErrorVO{
					Error:   "authz_unavailable",
					Message: "Unable to verify authorization",
				})
				return
			}

			if !result.Allowed || !authz.HasRole(result.Roles, authz.RoleAdmin) {
				forbidden(c)
				return
			}

			c.Set(ActingUserSubKey, result.Sub)
			c.Next()
			return
		}

		expected := util.GetEnv(constants.INTERNAL_SERVICE_TOKEN, "")
		presented := c.GetHeader(internalServiceTokenHeader)
		actingUserSub := c.GetHeader(actingUserSubHeader)
		if expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(presented)) != 1 || actingUserSub == "" {
			unauthorized(c)
			return
		}

		c.Set(ActingUserSubKey, actingUserSub)
		c.Next()
	}
}

func bearerToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, bearerPrefix) {
		return ""
	}
	return strings.TrimPrefix(auth, bearerPrefix)
}

func unauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, apiv.ErrorVO{
		Error:   "unauthorized",
		Message: "missing or invalid credentials",
	})
}

func forbidden(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusForbidden, apiv.ErrorVO{
		Error:   "forbidden",
		Message: "caller does not have a qualifying role",
	})
}
