package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/admin-api/constants"
	"github.com/sweetrpg/admin-api/feedback"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
)

// feedbackTypeBug and feedbackTypeFeature are the only accepted values for createFeedbackRequest.Type.
const (
	feedbackTypeBug     = "bug"
	feedbackTypeFeature = "feature"
)

// createFeedbackRequest is the payload accepted by POST /feedback. Website and RenderedAt back
// the bot-detection check (honeypot + minimum fill-time); real users never populate Website
// and never submit faster than FeedbackMinFillTime after RenderedAt - see design.md.
type createFeedbackRequest struct {
	Type          string     `json:"type" binding:"required" example:"bug"`
	Title         string     `json:"title" binding:"required" example:"Catalog search returns no results for exact ISBN match"`
	Body          string     `json:"body" binding:"required" example:"Searching for 9780786965601 returns zero results, even though the volume exists in the catalog."`
	ReporterEmail string     `json:"reporter_email,omitempty" example:"reporter@example.com"`
	Source        string     `json:"source,omitempty" example:"catalog-web:/catalog/search"`
	Website       string     `json:"website,omitempty"`
	RenderedAt    *time.Time `json:"rendered_at,omitempty"`
}

// feedbackAcceptedResponse is the uniform success body - identical whether a GitHub Issue was
// actually created or the submission was silently discarded as bot traffic. Deliberately gives
// a bot no signal about which outcome occurred.
type feedbackAcceptedResponse struct {
	Status string `json:"status" example:"accepted"`
}

func setupFeedbackHandlers(g *gin.Engine, issueCreator feedback.IssueCreator, rateLimit gin.HandlerFunc) {
	logging.Logger.Info("Setting up feedback endpoint handlers...")

	g.POST("/feedback", rateLimit, createFeedback(issueCreator))
}

// Submit feedback.
//
//	@Summary		Submit feedback
//	@Description	Accept an anonymous bug report or feature request and file it as a GitHub Issue on sweetrpg/platform, labeled source:in-app-feedback plus bug or feature. No GitHub credentials are ever requested from or exposed to the caller. Subject to a feedback-specific rate limit stricter than this service's default.
//	@Tags			feedback
//	@Accept			json
//	@Produce		json
//	@Param			body	body		createFeedbackRequest	true	"Feedback submission"
//	@Success		200		{object}	feedbackAcceptedResponse
//	@Failure		400		{object}	apiv.ErrorVO	"missing/invalid type, title, or body, or title/body exceeds the length cap"
//	@Failure		429		{object}	apiv.ErrorVO	"feedback-specific rate limit exceeded"
//	@Failure		502		{object}	apiv.ErrorVO	"GitHub API call failed or timed out"
//	@Failure		503		{object}	apiv.ErrorVO	"rate-limit backend unavailable"
//	@Router			/feedback [post]
func createFeedback(issueCreator feedback.IssueCreator) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createFeedbackRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
			return
		}

		if req.Type != feedbackTypeBug && req.Type != feedbackTypeFeature {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_type", Message: "type must be \"bug\" or \"feature\""})
			return
		}
		if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Body) == "" {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "title and body are required"})
			return
		}
		if len(req.Title) > constants.FeedbackTitleMaxLength {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "title_too_long", Message: fmt.Sprintf("title exceeds %d characters", constants.FeedbackTitleMaxLength)})
			return
		}
		if len(req.Body) > constants.FeedbackBodyMaxLength {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "body_too_long", Message: fmt.Sprintf("body exceeds %d characters", constants.FeedbackBodyMaxLength)})
			return
		}

		if looksAutomated(req) {
			logging.Logger.Warn("Discarding feedback submission that failed bot-detection check")
			c.JSON(http.StatusOK, feedbackAcceptedResponse{Status: "accepted"})
			return
		}

		body := issueBody(req)
		labels := []string{constants.FeedbackSourceLabel, req.Type}
		if err := issueCreator.CreateIssue(c.Request.Context(), req.Title, body, labels); err != nil {
			logging.Logger.Error("Failed to create GitHub issue from feedback submission", "error", err.Error())
			c.JSON(http.StatusBadGateway, apiv.ErrorVO{Error: "github_unavailable", Message: "failed to record feedback; please try again shortly"})
			return
		}

		c.JSON(http.StatusOK, feedbackAcceptedResponse{Status: "accepted"})
	}
}

// looksAutomated reports whether a submission fails the bot-detection check: a populated
// honeypot field (real users never see or fill it), or submission faster than
// constants.FeedbackMinFillTime after the client-reported render time.
func looksAutomated(req createFeedbackRequest) bool {
	if req.Website != "" {
		return true
	}
	if req.RenderedAt != nil && time.Since(*req.RenderedAt) < constants.FeedbackMinFillTime {
		return true
	}
	return false
}

// issueBody renders the submitted body plus any optional fields, appended only when present
// per the "Submission without optional fields" scenario - no empty placeholders.
func issueBody(req createFeedbackRequest) string {
	var b strings.Builder
	b.WriteString(req.Body)
	if req.ReporterEmail != "" {
		fmt.Fprintf(&b, "\n\n---\nReporter email: %s", req.ReporterEmail)
	}
	if req.Source != "" {
		fmt.Fprintf(&b, "\n\nSource: %s", req.Source)
	}
	return b.String()
}
