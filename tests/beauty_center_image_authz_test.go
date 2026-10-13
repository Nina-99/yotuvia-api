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

// beautyCenterImageTestRouter mounts BeautyCenterImageRoutes the way router.go
// does, over the shared SQLite schema.
func beautyCenterImageTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	r := gin.New()
	r.Use(gin.Recovery())
	router.BeautyCenterImageRoutes(r.Group("/api"), db, validator.New())
	return r, db
}

// seedImageFixtures creates center 1 (owner) and center 2 (victim) plus one
// gallery row each, so an IDOR probe has a real foreign row to attack.
func seedImageFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedDashboardCenter(t, db, 1, "Image Center", "image@test.com", true)
	seedDashboardCenter(t, db, 2, "Other Image Center", "otherimage@test.com", true)
	if err := db.Create(&models.BeautyCenterImage{
		Model: gorm.Model{ID: 1}, BeautyCenterID: 1, ImageUrl: "https://cdn.test/1.jpg",
	}).Error; err != nil {
		t.Fatalf("failed to seed image 1: %v", err)
	}
	if err := db.Create(&models.BeautyCenterImage{
		Model: gorm.Model{ID: 2}, BeautyCenterID: 2, ImageUrl: "https://cdn.test/2.jpg",
	}).Error; err != nil {
		t.Fatalf("failed to seed image 2: %v", err)
	}
}

// TestBeautyCenterImageAuthz_NoTokenIs401 — every /beauty-image route must
// reject anonymous callers.
func TestBeautyCenterImageAuthz_NoTokenIs401(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := beautyCenterImageTestRouter(t)
	seedImageFixtures(t, db)

	routes := []struct{ name, method, path string }{
		{"create", http.MethodPost, "/api/beauty-image"},
		{"update", http.MethodPatch, "/api/beauty-image/2"},
		{"delete", http.MethodDelete, "/api/beauty-image/2"},
	}
	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			w := do(r, rt.method, rt.path, "", "")
			assert.Equal(t, http.StatusUnauthorized, w.Code,
				"%s %s must reject anonymous callers", rt.method, rt.path)
			assert.Equal(t, "Authorization header missing", errBody(t, w.Body.String()))
		})
	}
}

// TestBeautyCenterImageAuthz_ClientRoleIs403 — a `client` token must not manage
// gallery rows; the role gate fires before the ownership check.
func TestBeautyCenterImageAuthz_ClientRoleIs403(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := beautyCenterImageTestRouter(t)
	seedImageFixtures(t, db)

	routes := []struct{ name, method, path, body string }{
		{"create", http.MethodPost, "/api/beauty-image", `{"beauty_center_id":1,"image_url":"https://cdn.test/x.jpg"}`},
		{"update", http.MethodPatch, "/api/beauty-image/2", `{"is_cover":true}`},
		{"delete", http.MethodDelete, "/api/beauty-image/2", ""},
	}
	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			w := do(r, rt.method, rt.path, roleToken(t, 9, 0, "client"), rt.body)
			assert.Equal(t, http.StatusForbidden, w.Code,
				"%s %s must be 403 for role client", rt.method, rt.path)
			assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
		})
	}
}

// TestBeautyCenterImage_CrossCenterForbidden — a center-1 admin must not
// update/delete center 2's gallery row; the foreign row must stay untouched,
// while center 2 can still manage its own image.
func TestBeautyCenterImage_CrossCenterForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := beautyCenterImageTestRouter(t)
	seedImageFixtures(t, db)

	t.Run("update another centre's image is 403", func(t *testing.T) {
		w := do(r, http.MethodPatch, "/api/beauty-image/2",
			roleToken(t, 1, 1, "beauty_center"), `{"is_cover":true,"position":9}`)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("delete another centre's image is 403", func(t *testing.T) {
		w := do(r, http.MethodDelete, "/api/beauty-image/2",
			roleToken(t, 1, 1, "beauty_center"), "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	// The foreign row must survive both attempts unmodified.
	var img models.BeautyCenterImage
	assert.NoError(t, db.First(&img, 2).Error, "another centre's image must not be deleted")
	assert.False(t, img.IsCover, "is_cover must not be set by a foreign admin")
	assert.Equal(t, 0, img.Position, "position must not be changed by a foreign admin")

	t.Run("owner updates its own image", func(t *testing.T) {
		w := do(r, http.MethodPatch, "/api/beauty-image/2",
			roleToken(t, 9, 2, "beauty_center"), `{"is_cover":true}`)

		assert.Equal(t, http.StatusOK, w.Code)
		var own models.BeautyCenterImage
		assert.NoError(t, db.First(&own, 2).Error)
		assert.True(t, own.IsCover)
	})

	t.Run("owner deletes its own image", func(t *testing.T) {
		w := do(r, http.MethodDelete, "/api/beauty-image/2",
			roleToken(t, 9, 2, "beauty_center"), "")

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestBeautyCenterImage_CreateScopedToCallerCenter — POST /beauty-image must
// ignore the body's beauty_center_id and bind the new row to the caller's
// centerID claim, exactly like loyalty promotions creation. A beauty_center
// token with no centerID claim cannot own an image, so it is rejected.
func TestBeautyCenterImage_CreateScopedToCallerCenter(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := beautyCenterImageTestRouter(t)
	seedDashboardCenter(t, db, 1, "Scoped Center", "scoped@test.com", true)
	seedDashboardCenter(t, db, 2, "Foreign Scoped Center", "foreignscoped@test.com", true)

	t.Run("body cannot create an image for another centre", func(t *testing.T) {
		w := do(r, http.MethodPost, "/api/beauty-image",
			roleToken(t, 1, 1, "beauty_center"),
			`{"beauty_center_id":2,"image_url":"https://cdn.test/scoped.jpg"}`)

		assert.Equal(t, http.StatusCreated, w.Code,
			"the request still succeeds — the row just must belong to the caller")

		var foreign int64
		assert.NoError(t, db.Model(&models.BeautyCenterImage{}).
			Where("beauty_center_id = ?", 2).Count(&foreign).Error)
		assert.Equal(t, int64(0), foreign,
			"a center-1 admin must never create a row owned by center 2")

		var owned int64
		assert.NoError(t, db.Model(&models.BeautyCenterImage{}).
			Where("beauty_center_id = ? AND image_url = ?", 1, "https://cdn.test/scoped.jpg").Count(&owned).Error)
		assert.Equal(t, int64(1), owned,
			"the image is persisted under the caller's own centre instead")
	})

	t.Run("beauty_center token without a centerID claim is rejected", func(t *testing.T) {
		w := do(r, http.MethodPost, "/api/beauty-image",
			roleToken(t, 1, 0, "beauty_center"),
			`{"beauty_center_id":1,"image_url":"https://cdn.test/orphan.jpg"}`)

		assert.Equal(t, http.StatusForbidden, w.Code,
			"a center admin with no centerID claim must not create an orphan image")
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))

		var orphan int64
		assert.NoError(t, db.Model(&models.BeautyCenterImage{}).
			Where("beauty_center_id = ?", 0).Count(&orphan).Error)
		assert.Equal(t, int64(0), orphan, "no zero-center image may be created")
	})
}
