package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/authz-client.go/authz"
	"github.com/sweetrpg/common.go/logging"
)

// TestWriteRoutesRequireAuth registers every route admin-api exposes (the
// same SetupHandlers wiring main.go uses) and asserts every write route
// (POST/PUT/DELETE) 401s without credentials, while every read route (GET) is
// reachable without them - none of these requests reach the database, since the
// middleware short-circuits unauthenticated writes and the unscoped GET request is
// rejected by listBanners' own validation before any query runs.
func TestWriteRoutesRequireAuth(t *testing.T) {
	logging.Init()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/profile" {
			_ = json.NewEncoder(w).Encode(map[string]string{"user_id": "test-user-id"})
			return
		}
		_ = json.NewEncoder(w).Encode(authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleAdmin}, Sub: "auth0|user-sub"})
	}))
	t.Cleanup(authAPI.Close)
	SetupHandlers(r, authz.NewClient(authAPI.URL, authAPI.URL))

	writeRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/banners"},
		{http.MethodPut, "/banners/000000000000000000000000"},
		{http.MethodDelete, "/banners/000000000000000000000000"},
	}
	for _, rt := range writeRoutes {
		t.Run(rt.method+" "+rt.path+" without headers", func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}

	t.Run("GET /banners without headers is not rejected as unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/banners", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized {
			t.Errorf("status = %d, read routes must not require write-auth headers", rec.Code)
		}
	})
}
