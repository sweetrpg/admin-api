package server

import (
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/admin-api/authz"
)

func SetupHandlers(g *gin.Engine, authzClient *authz.Client) {
	setupBannerHandlers(g, authzClient)
	setupMaintenanceModeHandlers(g, authzClient)
	setupAppCardStatusHandlers(g, authzClient)
	setupStatusHandlers(g)
}
