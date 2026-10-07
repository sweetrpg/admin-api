package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
	"github.com/sweetrpg/admin-api/feedback"
	"github.com/sweetrpg/common.go/logging"
)

func TestMain(m *testing.M) {
	logging.Init()
	m.Run()
}

// fakeIssueCreator records calls and returns a configurable error, so tests can assert both
// "an issue was created with these labels/title/body" and "GitHub failure surfaces as 502"
// without making a real network call.
type fakeIssueCreator struct {
	called bool
	title  string
	body   string
	labels []string
	err    error
}

func (f *fakeIssueCreator) CreateIssue(_ context.Context, title, body string, labels []string) error {
	f.called = true
	f.title = title
	f.body = body
	f.labels = labels
	return f.err
}

func newFeedbackTestRouter(creator feedback.IssueCreator) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	setupFeedbackHandlers(r, creator, func(c *gin.Context) { c.Next() })
	return r
}

func postFeedback(r *gin.Engine, body map[string]any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/feedback", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCreateFeedback_ValidBugReport(t *testing.T) {
	creator := &fakeIssueCreator{}
	r := newFeedbackTestRouter(creator)

	w := postFeedback(r, map[string]any{
		"type":  "bug",
		"title": "Catalog search broken",
		"body":  "Searching for an exact ISBN returns no results.",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	if !creator.called {
		t.Fatal("expected CreateIssue to be called for a valid submission")
	}
	if creator.title != "Catalog search broken" {
		t.Errorf("issue title = %q, want %q", creator.title, "Catalog search broken")
	}
	wantLabels := []string{"source:in-app-feedback", "bug"}
	if len(creator.labels) != 2 || creator.labels[0] != wantLabels[0] || creator.labels[1] != wantLabels[1] {
		t.Errorf("issue labels = %v, want %v", creator.labels, wantLabels)
	}
}

func TestCreateFeedback_MissingOrInvalidFields(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
	}{
		{"missing type", map[string]any{"title": "t", "body": "b"}},
		{"invalid type", map[string]any{"type": "complaint", "title": "t", "body": "b"}},
		{"missing title", map[string]any{"type": "bug", "body": "b"}},
		{"missing body", map[string]any{"type": "bug", "title": "t"}},
		{"blank title", map[string]any{"type": "bug", "title": "   ", "body": "b"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			creator := &fakeIssueCreator{}
			r := newFeedbackTestRouter(creator)

			w := postFeedback(r, tc.body)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
			}
			if creator.called {
				t.Error("expected CreateIssue not to be called for a rejected submission")
			}
		})
	}
}

func TestCreateFeedback_OversizedFieldsRejected(t *testing.T) {
	longTitle := make([]byte, 201)
	for i := range longTitle {
		longTitle[i] = 'a'
	}
	longBody := make([]byte, 10001)
	for i := range longBody {
		longBody[i] = 'b'
	}

	creator := &fakeIssueCreator{}
	r := newFeedbackTestRouter(creator)

	w := postFeedback(r, map[string]any{
		"type":  "feature",
		"title": string(longTitle),
		"body":  "ok",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("oversized title: status = %d, want 400", w.Code)
	}

	w = postFeedback(r, map[string]any{
		"type":  "feature",
		"title": "ok",
		"body":  string(longBody),
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("oversized body: status = %d, want 400", w.Code)
	}
	if creator.called {
		t.Error("expected CreateIssue not to be called for an oversized submission")
	}
}

func TestCreateFeedback_HoneypotPopulated_SilentlyAccepted(t *testing.T) {
	creator := &fakeIssueCreator{}
	r := newFeedbackTestRouter(creator)

	w := postFeedback(r, map[string]any{
		"type":    "bug",
		"title":   "t",
		"body":    "b",
		"website": "http://spam.example",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (silent accept); body = %s", w.Code, w.Body.String())
	}
	if creator.called {
		t.Error("expected CreateIssue not to be called when honeypot is populated")
	}
}

func TestCreateFeedback_SubmittedTooFast_SilentlyAccepted(t *testing.T) {
	creator := &fakeIssueCreator{}
	r := newFeedbackTestRouter(creator)

	renderedAt := time.Now()
	w := postFeedback(r, map[string]any{
		"type":        "bug",
		"title":       "t",
		"body":        "b",
		"rendered_at": renderedAt.Format(time.RFC3339Nano),
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (silent accept); body = %s", w.Code, w.Body.String())
	}
	if creator.called {
		t.Error("expected CreateIssue not to be called for a too-fast submission")
	}
}

func TestCreateFeedback_OptionalFieldsIncludedInBody(t *testing.T) {
	creator := &fakeIssueCreator{}
	r := newFeedbackTestRouter(creator)

	w := postFeedback(r, map[string]any{
		"type":           "feature",
		"title":          "t",
		"body":           "b",
		"reporter_email": "reporter@example.com",
		"source":         "main-web:/",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	if !creator.called {
		t.Fatal("expected CreateIssue to be called")
	}
	if !contains(creator.body, "reporter@example.com") || !contains(creator.body, "main-web:/") {
		t.Errorf("issue body = %q, want it to include both optional fields", creator.body)
	}
}

func TestCreateFeedback_OptionalFieldsOmittedWhenAbsent(t *testing.T) {
	creator := &fakeIssueCreator{}
	r := newFeedbackTestRouter(creator)

	w := postFeedback(r, map[string]any{"type": "bug", "title": "t", "body": "b"})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	if creator.body != "b" {
		t.Errorf("issue body = %q, want exactly the submitted body with no placeholders", creator.body)
	}
}

func TestCreateFeedback_GitHubFailure_Returns502(t *testing.T) {
	creator := &fakeIssueCreator{err: errors.New("github: 500 internal server error")}
	r := newFeedbackTestRouter(creator)

	w := postFeedback(r, map[string]any{"type": "bug", "title": "t", "body": "b"})

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body = %s", w.Code, w.Body.String())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}

// TestFeedbackRateLimit_ExceedsBudgetWithoutAffectingOtherRoutes exercises
// feedback.RateLimitMiddleware directly against a miniredis-backed pool: once the feedback
// tier's budget is exhausted, /feedback 429s while an unrelated route mounted alongside it
// keeps responding normally.
func TestFeedbackRateLimit_ExceedsBudgetWithoutAffectingOtherRoutes(t *testing.T) {
	t.Setenv("RATE_LIMIT_FEEDBACK", "1")
	t.Setenv("RATE_LIMIT_FEEDBACK_WINDOW_SECONDS", "60")

	mr := miniredis.RunT(t)
	pool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", mr.Addr())
		},
	}
	t.Cleanup(func() { _ = pool.Close() })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	creator := &fakeIssueCreator{}
	setupFeedbackHandlers(r, creator, feedback.RateLimitMiddleware(pool))
	r.GET("/other", func(c *gin.Context) { c.Status(http.StatusOK) })

	body := map[string]any{"type": "bug", "title": "t", "body": "b"}

	w1 := postFeedback(r, body)
	if w1.Code != http.StatusOK {
		t.Fatalf("first submission: status = %d, want 200; body = %s", w1.Code, w1.Body.String())
	}

	w2 := postFeedback(r, body)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second submission: status = %d, want 429; body = %s", w2.Code, w2.Body.String())
	}

	other := httptest.NewRequest(http.MethodGet, "/other", nil)
	otherW := httptest.NewRecorder()
	r.ServeHTTP(otherW, other)
	if otherW.Code != http.StatusOK {
		t.Errorf("unrelated route status = %d, want 200 - feedback rate limit must not affect other routes", otherW.Code)
	}
}
