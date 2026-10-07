package server

import (
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/admin-api/authz"
	"github.com/sweetrpg/admin-api/feedback"
)

func SetupHandlers(g *gin.Engine, authzClient *authz.Client, issueCreator feedback.IssueCreator, feedbackRateLimit gin.HandlerFunc) {
	setupBannerHandlers(g, authzClient)
	setupMaintenanceModeHandlers(g, authzClient)
	setupAppCardStatusHandlers(g, authzClient)
	setupStatusHandlers(g)
	setupFeedbackHandlers(g, issueCreator, feedbackRateLimit)
}
