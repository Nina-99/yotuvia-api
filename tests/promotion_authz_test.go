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

// promotionTestRouter mounts PromotionRoutes the way router.go does, over the
// shared SQLite schema. Recovery keeps a handler panic from crashing the binary
// so it surfaces as a 500 assertion failure instead.
func promotionTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	r := gin.New()
	r.Use(gin.Recovery())
	router.PromotionRoutes(r.Group("/api"), db, validator.New())
	return r, db
}

// seedPromotionFixtures creates center 1 (promo 1) and center 2 (promo 2) so an
// IDOR probe has a real foreign row to attack — an ownership check is only
// meaningful when the target actually belongs to somebody else.
func seedPromotionFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedDashboardCenter(t, db, 1, "Promo Center", "promo@test.com", true)
	seedDashboardCenter(t, db, 2, "Other Promo Center", "otherpromo@test.com", true)
	if err := db.Create(&models.Promotion{
		Model:          gorm.Model{ID: 1},
		BeautyCenterID: 1,
		Name:           "Center One Promo",
		Type:           "percentage",
		DiscountValue:  10,
		TargetClients:  "all",
		BuyXCount:      1,
		BuyYCount:      1,
		IsActive:       true,
	}).Error; err != nil {
		t.Fatalf("failed to seed promotion 1: %v", err)
	}
	if err := db.Create(&models.Promotion{
		Model:          gorm.Model{ID: 2},
		BeautyCenterID: 2,
		Name:           "Center Two Promo",
		Type:           "percentage",
		DiscountValue:  20,
		TargetClients:  "all",
		BuyXCount:      1,
		BuyYCount:      1,
		IsActive:       true,
	}).Error; err != nil {
		t.Fatalf("failed to seed promotion 2: %v", err)
	}
}

type promotionRoute struct {
	name   string
	method string
	path   string
	body   string
}

// promotionAdminRoutes is the center-admin /promotions surface. Before this fix
// these routes sat behind JWTAuthMiddleware with NO role gate, so any
// authenticated token — including role `client` — could manage promotions.
func promotionAdminRoutes() []promotionRoute {
	return []promotionRoute{
		{"create", http.MethodPost, "/api/promotions", `{"name":"Probe","type":"percentage","discount_value":5}`},
		{"center list", http.MethodGet, "/api/promotions/center/1", ""},
		{"by id", http.MethodGet, "/api/promotions/1", ""},
		{"update", http.MethodPut, "/api/promotions/1", `{"name":"Probe"}`},
		{"delete", http.MethodDelete, "/api/promotions/1", ""},
	}
}

// TestPromotionAuthz_NoTokenIs401 — every admin /promotions route must reject
// anonymous callers with the JWT middleware's response. The client-facing
// GET /promotions/center/:centerId/applicable is a SEPARATE public route and
// must stay reachable without a token.
func TestPromotionAuthz_NoTokenIs401(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := promotionTestRouter(t)
	seedPromotionFixtures(t, db)

	for _, rt := range promotionAdminRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			w := do(r, rt.method, rt.path, "", rt.body)

			assert.Equal(t, http.StatusUnauthorized, w.Code,
				"%s %s must reject anonymous callers", rt.method, rt.path)
			assert.Equal(t, "Authorization header missing", errBody(t, w.Body.String()),
				"handler must not run for an anonymous request")
		})
	}

	t.Run("applicable stays public", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/promotions/center/1/applicable?service_id=1", "", "")

		assert.NotEqual(t, http.StatusUnauthorized, w.Code,
			"the client-facing applicable endpoint must stay public")
	})
}

// TestPromotionAuthz_AdminRoutesRequireBeautyCenter — the management operations
// are 403 for client and personnel tokens (role gate), and still reachable for
// a beauty_center token.
func TestPromotionAuthz_AdminRoutesRequireBeautyCenter(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := promotionTestRouter(t)
	seedPromotionFixtures(t, db)

	for _, rt := range promotionAdminRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			for _, role := range []string{"client", "personnel"} {
				w := do(r, rt.method, rt.path, roleToken(t, 1, 1, role), rt.body)
				assert.Equal(t, http.StatusForbidden, w.Code,
					"%s %s must be 403 for role %s", rt.method, rt.path, role)
				assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
			}

			w := do(r, rt.method, rt.path, roleToken(t, 1, 1, "beauty_center"), rt.body)
			assert.NotEqual(t, http.StatusUnauthorized, w.Code)
			assert.NotEqual(t, http.StatusForbidden, w.Code)
			assert.NotEqual(t, "forbidden", errBody(t, w.Body.String()))
		})
	}
}

// TestPromotion_CreateScopedToCallerCenter — POST /promotions must ignore the
// body's beauty_center_id and bind the new promo to the caller's centerID
// claim, exactly like loyalty. A beauty_center token with no centerID claim
// cannot own a promo, so the request is rejected with 403 rather than
// persisting a zero-center orphan.
func TestPromotion_CreateScopedToCallerCenter(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := promotionTestRouter(t)
	seedPromotionFixtures(t, db)

	t.Run("body cannot create a promo for another center", func(t *testing.T) {
		w := do(r, http.MethodPost, "/api/promotions",
			roleToken(t, 1, 1, "beauty_center"),
			`{"beauty_center_id":2,"name":"Sneaky","type":"percentage","discount_value":5,"buy_x_count":1,"buy_y_count":1}`)

		assert.Equal(t, http.StatusCreated, w.Code,
			"the request still succeeds — the row just must belong to the caller")

		var foreign int64
		assert.NoError(t, db.Model(&models.Promotion{}).
			Where("beauty_center_id = ? AND name = ?", 2, "Sneaky").Count(&foreign).Error)
		assert.Equal(t, int64(0), foreign,
			"a center-1 admin must never create a row owned by center 2")

		var owned int64
		assert.NoError(t, db.Model(&models.Promotion{}).
			Where("beauty_center_id = ? AND name = ?", 1, "Sneaky").Count(&owned).Error)
		assert.Equal(t, int64(1), owned,
			"the promo is persisted under the caller's own center instead")
	})

	t.Run("beauty_center token without a centerID claim is rejected", func(t *testing.T) {
		w := do(r, http.MethodPost, "/api/promotions",
			roleToken(t, 1, 0, "beauty_center"),
			`{"name":"Orphan","type":"percentage","discount_value":5,"buy_x_count":1,"buy_y_count":1}`)

		assert.Equal(t, http.StatusForbidden, w.Code,
			"a center admin with no centerID claim must not create an orphan promo")
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))

		var orphan int64
		assert.NoError(t, db.Model(&models.Promotion{}).
			Where("beauty_center_id = ?", 0).Count(&orphan).Error)
		assert.Equal(t, int64(0), orphan, "no zero-center promo may be created")
	})
}

// TestPromotion_CrossCenterForbidden — RequireRole proves the ROLE, not
// OWNERSHIP. A center-1 admin must not read/update/delete a center-2 promo, and
// center 2's row must be untouched after the attempts. The owner still works.
func TestPromotion_CrossCenterForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := promotionTestRouter(t)
	seedPromotionFixtures(t, db)

	t.Run("update another center's promo is 403", func(t *testing.T) {
		w := do(r, http.MethodPut, "/api/promotions/2",
			roleToken(t, 1, 1, "beauty_center"), `{"name":"Hijacked"}`)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("delete another center's promo is 403", func(t *testing.T) {
		w := do(r, http.MethodDelete, "/api/promotions/2",
			roleToken(t, 1, 1, "beauty_center"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("read another center's promo is 403", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/promotions/2",
			roleToken(t, 1, 1, "beauty_center"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
		assert.NotContains(t, w.Body.String(), "Center Two Promo",
			"no promo data may leak to the rejected caller")
	})

	// The foreign row must survive all attempts unmodified.
	var p models.Promotion
	assert.NoError(t, db.First(&p, 2).Error, "another center's promo must not be deleted")
	assert.Equal(t, "Center Two Promo", p.Name, "name must not be modified by a foreign admin")

	t.Run("owner center manages its own promo", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/promotions/2", roleToken(t, 9, 2, "beauty_center"), "")
		assert.Equal(t, http.StatusOK, w.Code)

		w = do(r, http.MethodPut, "/api/promotions/2", roleToken(t, 9, 2, "beauty_center"), `{"name":"Renamed"}`)
		assert.Equal(t, http.StatusOK, w.Code)

		w = do(r, http.MethodDelete, "/api/promotions/2", roleToken(t, 9, 2, "beauty_center"), "")
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

// TestPromotion_FindByCenterCrossCenterForbidden — GET
// /promotions/center/:centerId must reject a path value that does not match the
// caller's centerID claim, while the owner reads its own list unchanged.
func TestPromotion_FindByCenterCrossCenterForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := promotionTestRouter(t)
	seedPromotionFixtures(t, db)

	w := do(r, http.MethodGet, "/api/promotions/center/2", roleToken(t, 1, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusForbidden, w.Code, "foreign center promo list must be 403")
	assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	assert.NotContains(t, w.Body.String(), "Center Two Promo")

	w = do(r, http.MethodGet, "/api/promotions/center/1", roleToken(t, 1, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusOK, w.Code, "the owner still lists its own promos")
	assert.Contains(t, w.Body.String(), "Center One Promo")
}
