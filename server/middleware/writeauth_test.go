package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/authz-client.go/authz"
	"github.com/sweetrpg/common.go/logging"
)

func init() {
	logging.Init()
}

func newWriteAuthTestRouter(authzBaseURL, usersBaseURL string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/write", WriteAuth(authz.NewClient(authzBaseURL, usersBaseURL)), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"viewer": authz.Viewer(c)})
	})
	return r
}

func newAuthzStub(t *testing.T, authzResponse authz.CheckResponse, authzStatus int, userID string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/profile" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"user_id": userID})
			return
		}
		w.WriteHeader(authzStatus)
		_ = json.NewEncoder(w).Encode(authzResponse)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func doBearerRequest(r *gin.Engine, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/write", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestWriteAuth_BearerToken_AcceptsAdminRole(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleAdmin}, Sub: "auth0|user-sub"}, http.StatusOK, "test-user-id")
	r := newWriteAuthTestRouter(srv.URL, srv.URL)

	rec := doBearerRequest(r, "good-token")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != `{"viewer":"test-user-id"}` {
		t.Errorf("body = %q, want canonical user id forwarded to the handler", got)
	}
}

func TestWriteAuth_BearerToken_RejectsWithoutAdminRole(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleEditor}, Sub: "auth0|user-sub"}, http.StatusOK, "test-user-id")
	r := newWriteAuthTestRouter(srv.URL, srv.URL)

	rec := doBearerRequest(r, "good-token")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestWriteAuth_BearerToken_RejectsInvalidToken(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{}, http.StatusUnauthorized, "")
	r := newWriteAuthTestRouter(srv.URL, srv.URL)

	rec := doBearerRequest(r, "bad-token")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestWriteAuth_RejectsMissingCredentials(t *testing.T) {
	r := newWriteAuthTestRouter("", "")

	rec := doBearerRequest(r, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestWriteAuth_RejectsUnresolvableUser(t *testing.T) {
	// Auth succeeds but users-api returns 404 for the subject
	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleAdmin}, Sub: "auth0|unknown"}, http.StatusOK, "")
	r := newWriteAuthTestRouter(srv.URL, srv.URL)

	rec := doBearerRequest(r, "good-token")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
