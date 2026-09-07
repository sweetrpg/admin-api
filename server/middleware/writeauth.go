// Package middleware provides Gin middleware for admin-api's route groups.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/authz-client.go/authz"
)

// WriteAuth requires a forwarded user bearer token carrying the admin role, verified
// against auth-api's /authz/check. On success, the resolved canonical user ID is
// stashed in the context (read via authz.Viewer(c)) for handlers to attribute their
// audit records to. Fails closed if the acting user cannot be resolved to a canonical ID.
func WriteAuth(client *authz.Client) gin.HandlerFunc {
	return authz.RequireAnyRole(client, "admin-api", authz.RoleAdmin)
}
