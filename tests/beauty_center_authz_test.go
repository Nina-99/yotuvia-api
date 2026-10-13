package tests

import (
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// beautyCenterTestRouter mounts BeautyCenterRoutes the way router.go does, over
// the shared SQLite schema. Recovery keeps a handler panic from crashing the
// binary so it surfaces as a 500 assertion failure instead.
func beautyCenterTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	r := gin.New()
	r.Use(gin.Recovery())
	router.BeautyCenterRoutes(r.Group("/api"), db, validator.New())
	return r, db
}

// seedBeautyCenterFixtures creates center 1 (owner) and center 2 (victim) so a
// cross-tenant probe has a real foreign row to attack.
func seedBeautyCenterFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedDashboardCenter(t, db, 1, "Owner Center", "owner@test.com", true)
	seedDashboardCenter(t, db, 2, "Victim Center", "victim@test.com", true)
}

// beautyCenterWriteRoutes is the protected PUT/DELETE surface. Before this fix
// the block applied only JWTAuthMiddleware, so ANY authenticated token —
// including role `client` — could rewrite another centre.
func beautyCenterWriteRoutes() []struct{ name, method, path string } {
	return []struct{ name, method, path string }{
		{"update", http.MethodPut, "/api/beauty-centers/2"},
		{"delete", http.MethodDelete, "/api/beauty-centers/2"},
	}
}

// TestBeautyCenterAuthz_NoTokenIs401 — PUT/DELETE must reject anonymous callers
// with the JWT middleware's response, never reach the handler.
func TestBeautyCenterAuthz_NoTokenIs401(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := beautyCenterTestRouter(t)
	seedBeautyCenterFixtures(t, db)

	for _, rt := range beautyCenterWriteRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			w := do(r, rt.method, rt.path, "", `{"name":"Hijacked"}`)

			assert.Equal(t, http.StatusUnauthorized, w.Code,
				"%s %s must reject anonymous callers", rt.method, rt.path)
			assert.Equal(t, "Authorization header missing", errBody(t, w.Body.String()),
				"handler must not run for an anonymous request")
		})
	}
}

// TestBeautyCenterAuthz_ClientRoleIs403 — a `client` token must not manage
// beauty centres at all; the role gate is the first line of defence.
func TestBeautyCenterAuthz_ClientRoleIs403(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := beautyCenterTestRouter(t)
	seedBeautyCenterFixtures(t, db)

	for _, rt := range beautyCenterWriteRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			w := do(r, rt.method, rt.path, roleToken(t, 9, 0, "client"), `{"name":"Hijacked"}`)

			assert.Equal(t, http.StatusForbidden, w.Code,
				"%s %s must be 403 for role client", rt.method, rt.path)
			assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
		})
	}
}

// TestBeautyCenter_CrossTenantTakeoverForbidden — CRITICAL IDOR. RequireRole
// proves the ROLE, not OWNERSHIP: a center-1 admin must not rewrite or delete
// center 2 (including its email and password), and center 2 must survive
// byte-for-byte. The owning centre still works.
func TestBeautyCenter_CrossTenantTakeoverForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := beautyCenterTestRouter(t)
	seedBeautyCenterFixtures(t, db)

	var before models.BeautyCenter
	assert.NoError(t, db.First(&before, 2).Error)

	const takeover = `{"name":"Hijacked","email":"hijack@test.com","password":"pwned-pass-123"}`

	t.Run("foreign admin update is 403", func(t *testing.T) {
		w := do(r, http.MethodPut, "/api/beauty-centers/2", roleToken(t, 1, 1, "beauty_center"), takeover)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("foreign admin delete is 403", func(t *testing.T) {
		w := do(r, http.MethodDelete, "/api/beauty-centers/2", roleToken(t, 1, 1, "beauty_center"), "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	// The victim row must survive both attempts, credentials included.
	var after models.BeautyCenter
	assert.NoError(t, db.First(&after, 2).Error, "another centre must not be deleted")
	assert.Equal(t, before.Name, after.Name)
	assert.Equal(t, before.Email, after.Email, "email must not be rewritten by a foreign admin")
	assert.Equal(t, before.Password, after.Password, "password must not be rewritten by a foreign admin")

	t.Run("owner updates its own centre", func(t *testing.T) {
		w := do(r, http.MethodPut, "/api/beauty-centers/2", roleToken(t, 9, 2, "beauty_center"),
			`{"name":"Renamed Victim"}`)

		assert.Equal(t, http.StatusOK, w.Code)
		var own models.BeautyCenter
		assert.NoError(t, db.First(&own, 2).Error)
		assert.Equal(t, "Renamed Victim", own.Name)
	})

	t.Run("owner deletes its own centre", func(t *testing.T) {
		w := do(r, http.MethodDelete, "/api/beauty-centers/2", roleToken(t, 9, 2, "beauty_center"), "")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "deleted successfully")
	})
}

// TestBeautyCenter_TokenWithoutCenterClaimIs403 — a beauty_center token with no
// centerID claim (0) can never own a path id, so it is rejected even against a
// row that exists, rather than being allowed to manage every centre.
func TestBeautyCenter_TokenWithoutCenterClaimIs403(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := beautyCenterTestRouter(t)
	seedBeautyCenterFixtures(t, db)

	w := do(r, http.MethodPut, "/api/beauty-centers/1", roleToken(t, 9, 0, "beauty_center"), `{"name":"Hijacked"}`)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "forbidden", errBody(t, w.Body.String()))

	var c models.BeautyCenter
	assert.NoError(t, db.First(&c, 1).Error)
	assert.Equal(t, "Owner Center", c.Name)
}
