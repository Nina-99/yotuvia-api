package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Nina-99/yotuvia-api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// securityHeadersRouter mounts a probe handler behind the real middleware.
func securityHeadersRouter(t *testing.T, maxBody int64) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.BodyLimit(maxBody))
	r.POST("/probe", func(ctx *gin.Context) {
		var body map[string]any
		if err := ctx.ShouldBindJSON(&body); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

// TestSecurityHeaders_Present — every response carries the hardening headers,
// including the CSP that neutralizes a file served from /uploads.
func TestSecurityHeaders_Present(t *testing.T) {
	r := securityHeadersRouter(t, 1024)

	req, _ := http.NewRequest(http.MethodPost, "/probe", strings.NewReader(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
	assert.Contains(t, w.Header().Get("Content-Security-Policy"), "default-src 'none'")
	assert.Contains(t, w.Header().Get("Content-Security-Policy"), "sandbox")
	assert.Empty(t, w.Header().Get("Strict-Transport-Security"),
		"HSTS must not be advertised over plain HTTP")
}

// TestSecurityHeaders_HSTSOnlyOverTLS — behind a TLS-terminating proxy the
// scheme arrives in X-Forwarded-Proto and HSTS is then emitted.
func TestSecurityHeaders_HSTSOnlyOverTLS(t *testing.T) {
	r := securityHeadersRouter(t, 1024)

	req, _ := http.NewRequest(http.MethodPost, "/probe", strings.NewReader(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "max-age=31536000")
}

// TestBodyLimit_RejectsOversizedJSON — an oversized body is refused before it
// is buffered, and multipart uploads stay exempt so real uploads keep working.
func TestBodyLimit_RejectsOversizedJSON(t *testing.T) {
	r := securityHeadersRouter(t, 32)

	big := `{"payload":"` + strings.Repeat("x", 64) + `"}`
	req, _ := http.NewRequest(http.MethodPost, "/probe", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Contains(t, w.Body.String(), "request body too large")

	// A small body still goes through untouched.
	req2, _ := http.NewRequest(http.MethodPost, "/probe", strings.NewReader(`{"a":1}`))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}
