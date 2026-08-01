package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newWriteAuthTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/write", WriteAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"actingUserSub": c.GetString(ActingUserSubKey)})
	})
	return r
}

func doRequest(r *gin.Engine, token, actingUserSub string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/write", nil)
	if token != "" {
		req.Header.Set(internalServiceTokenHeader, token)
	}
	if actingUserSub != "" {
		req.Header.Set(actingUserSubHeader, actingUserSub)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestWriteAuth_RejectsWhenTokenUnset(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_TOKEN", "")
	r := newWriteAuthTestRouter()

	rec := doRequest(r, "anything", "user-sub")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestWriteAuth_RejectsMissingToken(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_TOKEN", "secret")
	r := newWriteAuthTestRouter()

	rec := doRequest(r, "", "user-sub")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestWriteAuth_RejectsInvalidToken(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_TOKEN", "secret")
	r := newWriteAuthTestRouter()

	rec := doRequest(r, "wrong-token", "user-sub")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestWriteAuth_RejectsMissingActingUserSub(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_TOKEN", "secret")
	r := newWriteAuthTestRouter()

	rec := doRequest(r, "secret", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestWriteAuth_AcceptsValidCombination(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_TOKEN", "secret")
	r := newWriteAuthTestRouter()

	rec := doRequest(r, "secret", "auth0|user-sub")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != `{"actingUserSub":"auth0|user-sub"}` {
		t.Errorf("body = %q, want acting user sub to be forwarded to the handler", got)
	}
}
