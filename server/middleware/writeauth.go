// Package middleware provides Gin middleware for admin-api's route groups.
package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/admin-api/constants"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/util"
)

const (
	internalServiceTokenHeader = "X-Internal-Service-Token"
	actingUserSubHeader        = "X-Acting-User-Sub"

	// ActingUserSubKey is the gin.Context key write handlers read to get the
	// attributed acting user after WriteAuth has validated the request.
	ActingUserSubKey = "actingUserSub"
)

// WriteAuth requires a valid X-Internal-Service-Token header (compared
// constant-time against INTERNAL_SERVICE_TOKEN) and a non-empty
// X-Acting-User-Sub header, matching users-api's InternalServiceAuth contract
// for the same caller (admin-web). An unset or empty INTERNAL_SERVICE_TOKEN
// permanently disables the write path rather than trusting an empty token.
// On success, the acting user is stashed on the context under
// ActingUserSubKey for handlers to attribute their audit records to.
func WriteAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		expected := util.GetEnv(constants.INTERNAL_SERVICE_TOKEN, "")
		presented := c.GetHeader(internalServiceTokenHeader)
		actingUserSub := c.GetHeader(actingUserSubHeader)

		if expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(presented)) != 1 || actingUserSub == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apiv.ErrorVO{
				Error:   "unauthorized",
				Message: "missing or invalid internal service credentials",
			})
			return
		}

		c.Set(ActingUserSubKey, actingUserSub)
		c.Next()
	}
}
