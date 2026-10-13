package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Nina-99/yotuvia-api/middleware"
	"github.com/gin-gonic/gin"
)

// The web app runs on a different origin than the API, so the browser sends a
// CORS preflight (OPTIONS) before POSTing JSON with an Authorization header.
// Gin has no OPTIONS route registered, so before this middleware existed the
// preflight fell through to 404 and the login request never even reached the
// handler.
func corsTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(middleware.CORS())
	r.POST("/api/auth-accounts/login", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestCORS_PreflightOnUnroutedOptionsIsNot404(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")
	r := corsTestRouter(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/auth-accounts/login", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code == http.StatusNotFound {
		t.Fatalf("preflight must not fall through to 404, got %d", res.Code)
	}
	if res.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", res.Code)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want the request origin", got)
	}
	methods := res.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "POST") {
		t.Fatalf("Access-Control-Allow-Methods = %q, want it to allow POST", methods)
	}
	headers := strings.ToLower(res.Header().Get("Access-Control-Allow-Headers"))
	if !strings.Contains(headers, "authorization") {
		t.Fatalf("Access-Control-Allow-Headers = %q, want it to allow Authorization", headers)
	}
	if vary := res.Header().Get("Vary"); !strings.Contains(vary, "Origin") {
		t.Fatalf("Vary = %q, want it to include Origin", vary)
	}
}

func TestCORS_AllowedOriginGetsHeaderOnActualRequest(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")
	r := corsTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth-accounts/login", strings.NewReader(`{}`))
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want the request origin", got)
	}
}

func TestCORS_DisallowedOriginGetsNoAllowOriginHeader(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")
	r := corsTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth-accounts/login", strings.NewReader(`{}`))
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want it empty for a disallowed origin", got)
	}
}

func TestCORS_FallsBackToFrontHost(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("FRONT_HOST", "http://localhost:4321")
	r := corsTestRouter(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/auth-accounts/login", nil)
	req.Header.Set("Origin", "http://localhost:4321")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:4321" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want the FRONT_HOST fallback", got)
	}
}

func TestCORS_AcceptsMultipleOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173, https://app.yotuvia.app")
	r := corsTestRouter(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/auth-accounts/login", nil)
	req.Header.Set("Origin", "https://app.yotuvia.app")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "https://app.yotuvia.app" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want the second configured origin", got)
	}
}
