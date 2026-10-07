package feedback

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-github/v76/github"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *GitHubIssueClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	gh := github.NewClient(nil).WithAuthToken("test-token")
	base, err := url.Parse(server.URL + "/")
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	gh.BaseURL = base

	return &GitHubIssueClient{client: gh}
}

func TestCreateIssue_Success(t *testing.T) {
	var gotPath string
	var gotBody github.IssueRequest

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(github.Issue{Number: github.Ptr(1)})
	})

	err := client.CreateIssue(context.Background(), "Bug: thing broke", "Steps to reproduce...", []string{"source:in-app-feedback", "bug"})
	if err != nil {
		t.Fatalf("CreateIssue returned error: %v", err)
	}

	wantPath := "/repos/" + Owner + "/" + Repo + "/issues"
	if gotPath != wantPath {
		t.Errorf("request path = %q, want %q", gotPath, wantPath)
	}
	if gotBody.Title == nil || *gotBody.Title != "Bug: thing broke" {
		t.Errorf("request title = %v, want %q", gotBody.Title, "Bug: thing broke")
	}
	if gotBody.Labels == nil || len(*gotBody.Labels) != 2 || (*gotBody.Labels)[0] != "source:in-app-feedback" || (*gotBody.Labels)[1] != "bug" {
		t.Errorf("request labels = %v, want [source:in-app-feedback bug]", gotBody.Labels)
	}
}

func TestCreateIssue_GitHubError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"internal error"}`))
	})

	err := client.CreateIssue(context.Background(), "Bug", "Body", []string{"source:in-app-feedback", "bug"})
	if err == nil {
		t.Fatal("CreateIssue returned nil error, want non-nil on GitHub API failure")
	}
}
