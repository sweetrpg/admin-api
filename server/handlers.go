package server

import (
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/admin-api/feedback"
	"github.com/sweetrpg/authz-client.go/authz"
)

func SetupHandlers(g *gin.Engine, authzClient *authz.Client, issueCreator feedback.IssueCreator, feedbackRateLimit gin.HandlerFunc) {
	setupBannerHandlers(g, authzClient)
	setupMaintenanceModeHandlers(g, authzClient)
	setupAppCardStatusHandlers(g, authzClient)
	setupStatusHandlers(g)
	setupFeedbackHandlers(g, issueCreator, feedbackRateLimit)
}
