// Package feedback implements the anonymous-feedback-to-GitHub-Issue proxy: a single
// IssueCreator interface backed by the GitHub Issues API, kept separate from the HTTP handler
// in server/feedback.go so the handler can be unit-tested against a fake creator.
package feedback

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/go-github/v76/github"
)

// Owner and Repo identify the sweetrpg/platform meta-repo that every feedback submission
// becomes an issue on - see design.md's "no per-service repo routing" decision.
const (
	Owner = "sweetrpg"
	Repo  = "platform"

	// createIssueTimeout bounds the GitHub API call so a stalled connection fails fast and
	// the caller gets a prompt 502 rather than hanging past their own timeout.
	createIssueTimeout = 10 * time.Second
)

// IssueCreator creates a GitHub Issue from a feedback submission. Implemented by
// GitHubIssueClient; tests substitute a fake.
type IssueCreator interface {
	CreateIssue(ctx context.Context, title, body string, labels []string) error
}

// GitHubIssueClient creates issues on sweetrpg/platform via the GitHub REST API using a
// service-owned token scoped to Issues:write on that one repo (see design.md).
type GitHubIssueClient struct {
	client *github.Client
}

// NewGitHubIssueClient builds a client authenticated with the given token.
func NewGitHubIssueClient(token string) *GitHubIssueClient {
	return &GitHubIssueClient{client: github.NewClient(nil).WithAuthToken(token)}
}

// CreateIssue creates a labeled issue on sweetrpg/platform. A non-nil error means the caller
// should treat the submission as failed (502/503) - this never retries internally, per
// design.md's "no partial state to reconcile" decision.
func (c *GitHubIssueClient) CreateIssue(ctx context.Context, title, body string, labels []string) error {
	ctx, cancel := context.WithTimeout(ctx, createIssueTimeout)
	defer cancel()

	_, resp, err := c.client.Issues.Create(ctx, Owner, Repo, &github.IssueRequest{
		Title:  &title,
		Body:   &body,
		Labels: &labels,
	})
	if err != nil {
		return fmt.Errorf("feedback: create github issue: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("feedback: create github issue: unexpected status %d", resp.StatusCode)
	}
	return nil
}
