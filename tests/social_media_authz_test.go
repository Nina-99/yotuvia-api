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

// socialMediaTestRouter mounts SocialMediaRoutes the way router.go does, over
// the shared SQLite schema.
func socialMediaTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	r := gin.New()
	r.Use(gin.Recovery())
	router.SocialMediaRoutes(r.Group("/api"), db, validator.New())
	return r, db
}

// seedSocialFixtures creates center 1 (owner) and center 2 (victim) plus one
// social-links row each. The SocialMedia model puts a unique index on every
// platform column, so each seeded field must carry a distinct non-empty value.
func seedSocialFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedDashboardCenter(t, db, 1, "Social Center", "social@test.com", true)
	seedDashboardCenter(t, db, 2, "Other Social Center", "othersocial@test.com", true)
	for _, row := range []models.SocialMedia{
		{Model: gorm.Model{ID: 1}, BeautyCenterID: 1,
			Google: "https://google.com/1", Facebook: "https://facebook.com/1",
			Instagram: "https://instagram.com/1", Twitter: "https://twitter.com/1", TikTok: "https://tiktok.com/1"},
		{Model: gorm.Model{ID: 2}, BeautyCenterID: 2,
			Google: "https://google.com/2", Facebook: "https://facebook.com/2",
			Instagram: "https://instagram.com/2", Twitter: "https://twitter.com/2", TikTok: "https://tiktok.com/2"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("failed to seed social media %d: %v", row.ID, err)
		}
	}
}

// TestSocialMediaAuthz_NoTokenIs401 — the protected /social-media routes must
// reject anonymous callers. The public GET /beauty-centers/:id/social-media is
// a separate route and must stay reachable without a token.
func TestSocialMediaAuthz_NoTokenIs401(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := socialMediaTestRouter(t)
	seedSocialFixtures(t, db)

	routes := []struct{ name, method, path string }{
		{"create", http.MethodPost, "/api/social-media"},
		{"update", http.MethodPut, "/api/social-media/2"},
		{"delete", http.MethodDelete, "/api/social-media/2"},
	}
	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			w := do(r, rt.method, rt.path, "", "")
			assert.Equal(t, http.StatusUnauthorized, w.Code,
				"%s %s must reject anonymous callers", rt.method, rt.path)
			assert.Equal(t, "Authorization header missing", errBody(t, w.Body.String()))
		})
	}

	t.Run("public by-center read stays public", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/beauty-centers/1/social-media", "", "")
		assert.Equal(t, http.StatusOK, w.Code,
			"the public social-media read must stay anonymous-reachable")
	})
}

// TestSocialMediaAuthz_ClientRoleIs403 — a `client` token must not manage
// social links.
func TestSocialMediaAuthz_ClientRoleIs403(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := socialMediaTestRouter(t)
	seedSocialFixtures(t, db)

	routes := []struct{ name, method, path, body string }{
		{"create", http.MethodPost, "/api/social-media", `{"beauty_center_id":1,"instagram":"https://instagram.com/x"}`},
		{"update", http.MethodPut, "/api/social-media/2", `{"instagram":"https://instagram.com/x"}`},
		{"delete", http.MethodDelete, "/api/social-media/2", ""},
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

// TestSocialMedia_CrossCenterForbidden — a center-1 admin must not update or
// delete center 2's social links (and vice versa). The foreign row must stay
// untouched while its owner keeps managing it.
func TestSocialMedia_CrossCenterForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := socialMediaTestRouter(t)
	seedSocialFixtures(t, db)

	t.Run("update another centre's links is 403", func(t *testing.T) {
		w := do(r, http.MethodPut, "/api/social-media/2",
			roleToken(t, 1, 1, "beauty_center"), `{"instagram":"https://instagram.com/hijacked"}`)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("delete another centre's links is 403", func(t *testing.T) {
		w := do(r, http.MethodDelete, "/api/social-media/2",
			roleToken(t, 1, 1, "beauty_center"), "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("reverse direction is also 403", func(t *testing.T) {
		w := do(r, http.MethodPut, "/api/social-media/1",
			roleToken(t, 9, 2, "beauty_center"), `{"instagram":"https://instagram.com/hijacked"}`)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	// The foreign row must survive all attempts unmodified.
	var sm models.SocialMedia
	assert.NoError(t, db.First(&sm, 2).Error, "another centre's links must not be deleted")
	assert.Equal(t, "https://instagram.com/2", sm.Instagram,
		"instagram must not be rewritten by a foreign admin")

	t.Run("owner updates its own links", func(t *testing.T) {
		w := do(r, http.MethodPut, "/api/social-media/2",
			roleToken(t, 9, 2, "beauty_center"), `{"instagram":"https://instagram.com/2renamed"}`)

		assert.Equal(t, http.StatusOK, w.Code)
		var own models.SocialMedia
		assert.NoError(t, db.First(&own, 2).Error)
		assert.Equal(t, "https://instagram.com/2renamed", own.Instagram)
	})

	t.Run("owner deletes its own links", func(t *testing.T) {
		w := do(r, http.MethodDelete, "/api/social-media/2",
			roleToken(t, 9, 2, "beauty_center"), "")

		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

// TestSocialMedia_CreateScopedToCallerCenter — POST /social-media must ignore
// the body's beauty_center_id and bind the new row to the caller's centerID
// claim. A beauty_center token with no centerID claim cannot own links, so it
// is rejected rather than persisting a zero-centre orphan.
func TestSocialMedia_CreateScopedToCallerCenter(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := socialMediaTestRouter(t)
	seedDashboardCenter(t, db, 1, "Scoped Social Center", "scopedsocial@test.com", true)
	seedDashboardCenter(t, db, 2, "Foreign Social Center", "foreignsocial@test.com", true)

	t.Run("body cannot create links for another centre", func(t *testing.T) {
		w := do(r, http.MethodPost, "/api/social-media",
			roleToken(t, 1, 1, "beauty_center"),
			`{"beauty_center_id":2,"instagram":"https://instagram.com/scoped"}`)

		assert.Equal(t, http.StatusCreated, w.Code,
			"the request still succeeds — the row just must belong to the caller")

		var foreign int64
		assert.NoError(t, db.Model(&models.SocialMedia{}).
			Where("beauty_center_id = ?", 2).Count(&foreign).Error)
		assert.Equal(t, int64(0), foreign,
			"a center-1 admin must never create links owned by center 2")

		var owned int64
		assert.NoError(t, db.Model(&models.SocialMedia{}).
			Where("beauty_center_id = ? AND instagram = ?", 1, "https://instagram.com/scoped").Count(&owned).Error)
		assert.Equal(t, int64(1), owned,
			"the links are persisted under the caller's own centre instead")
	})

	t.Run("beauty_center token without a centerID claim is rejected", func(t *testing.T) {
		w := do(r, http.MethodPost, "/api/social-media",
			roleToken(t, 1, 0, "beauty_center"),
			`{"beauty_center_id":1,"instagram":"https://instagram.com/orphan"}`)

		assert.Equal(t, http.StatusForbidden, w.Code,
			"a center admin with no centerID claim must not create orphan links")
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))

		var orphan int64
		assert.NoError(t, db.Model(&models.SocialMedia{}).
			Where("beauty_center_id = ?", 0).Count(&orphan).Error)
		assert.Equal(t, int64(0), orphan, "no zero-centre links may be created")
	})
}
