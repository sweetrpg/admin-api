package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/common.go/logging"
)

// TestWriteRoutesRequireAuth registers every route admin-api exposes (the
// same SetupHandlers wiring main.go uses) and asserts every write route
// (POST/PUT/DELETE) 401s without the internal-service-token/acting-user-sub
// headers, while every read route (GET) is reachable without them - none of
// these requests reach the database, since the middleware short-circuits
// unauthenticated writes and the unscoped GET request is rejected by
// listBanners' own validation before any query runs.
func TestWriteRoutesRequireAuth(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_TOKEN", "")
	logging.Init()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupHandlers(r)

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
