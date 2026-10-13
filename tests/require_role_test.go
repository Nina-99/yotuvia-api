package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nina-99/yotuvia-api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

const requireRoleForbiddenBody = `{"error":"forbidden"}`

// roleProbeRouter chains a claim stub (what JWTAuthMiddleware leaves in ctx)
// in front of RequireRole, mirroring the production chain order.
func roleProbeRouter(claims map[string]interface{}, allowed ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(ctx *gin.Context) {
		for k, v := range claims {
			ctx.Set(k, v)
		}
		ctx.Next()
	})
	r.Use(middleware.RequireRole(allowed...))
	r.GET("/probe", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestRequireRole(t *testing.T) {
	t.Run("allowed role lets handler run", func(t *testing.T) {
		r := roleProbeRouter(map[string]interface{}{"role": "beauty_center"}, "beauty_center")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/probe", nil))

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"ok":true`)
	})

	t.Run("any of several allowed roles lets handler run", func(t *testing.T) {
		r := roleProbeRouter(map[string]interface{}{"role": "client"}, "client", "beauty_center")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/probe", nil))

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"ok":true`)
	})

	t.Run("disallowed role is 403 and handler never runs", func(t *testing.T) {
		r := roleProbeRouter(map[string]interface{}{"role": "client"}, "beauty_center")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/probe", nil))

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, requireRoleForbiddenBody, w.Body.String())
	})

	t.Run("missing role claim is 403 and handler never runs", func(t *testing.T) {
		r := roleProbeRouter(map[string]interface{}{}, "beauty_center")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/probe", nil))

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, requireRoleForbiddenBody, w.Body.String())
	})

	t.Run("non-string role claim is 403 and handler never runs", func(t *testing.T) {
		r := roleProbeRouter(map[string]interface{}{"role": 42}, "beauty_center")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/probe", nil))

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, requireRoleForbiddenBody, w.Body.String())
	})
}
