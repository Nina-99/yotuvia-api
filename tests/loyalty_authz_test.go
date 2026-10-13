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

// loyaltyTestRouter mounts LoyaltyRoutes the way router.go does, over the shared
// SQLite schema. Recovery keeps a handler panic from crashing the binary so it
// surfaces as a 500 assertion failure instead.
func loyaltyTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	r := gin.New()
	r.Use(gin.Recovery())
	router.LoyaltyRoutes(r.Group("/api"), db, validator.New())
	return r, db
}

// seedLoyaltyFixtures creates center 1 with one program and one punch card
// owned by client 1, so allowed-role probes reach real rows.
func seedLoyaltyFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedDashboardCenter(t, db, 1, "Loyalty Center", "loyalty@test.com", true)
	if err := db.Create(&models.LoyaltyProgram{
		Model:          gorm.Model{ID: 1},
		BeautyCenterID: 1,
		RequiredSteps:  5,
		RewardType:     "free",
		IsActive:       true,
	}).Error; err != nil {
		t.Fatalf("failed to seed loyalty program: %v", err)
	}
	if err := db.Create(&models.CustomerPunchCard{
		Model:            gorm.Model{ID: 1},
		ClientID:         1,
		LoyaltyProgramID: 1,
		CurrentSteps:     3,
		Status:           "active",
	}).Error; err != nil {
		t.Fatalf("failed to seed punch card: %v", err)
	}
}

// seedSecondCenterWithProgram creates center 2 and its own program (id 2) so the
// cross-center probes have a real foreign row to attack. An IDOR probe is only
// meaningful when the target actually belongs to somebody else.
func seedSecondCenterWithProgram(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedDashboardCenter(t, db, 2, "Other Center", "other@test.com", true)
	if err := db.Create(&models.LoyaltyProgram{
		Model:          gorm.Model{ID: 2},
		BeautyCenterID: 2,
		RequiredSteps:  5,
		RewardType:     "free",
		IsActive:       true,
	}).Error; err != nil {
		t.Fatalf("failed to seed center-2 loyalty program: %v", err)
	}
}

type loyaltyRoute struct {
	name   string
	method string
	path   string
}

// loyaltyAllRoutes is the whole /loyalty surface. Before this fix NONE of these
// required a token, which is the vulnerability under test.
func loyaltyAllRoutes() []loyaltyRoute {
	return []loyaltyRoute{
		{"create program", http.MethodPost, "/api/loyalty"},
		{"client cards", http.MethodGet, "/api/loyalty/client/1"},
		{"center programs", http.MethodGet, "/api/loyalty/center/1"},
		{"center settings read", http.MethodGet, "/api/loyalty/center/1/settings"},
		{"center settings update", http.MethodPut, "/api/loyalty/center/1/settings"},
		{"update program", http.MethodPut, "/api/loyalty/1"},
		{"delete program", http.MethodDelete, "/api/loyalty/1"},
		{"toggle program", http.MethodPatch, "/api/loyalty/1/toggle"},
	}
}

// loyaltyAdminOnlyRoutes are writes plus center reads: beauty_center only.
// GET /loyalty/center/:centerId is admin-only because its sole consumers are the
// admin API clients (see use-loyalty hook / admin-api.ts). GET
// /loyalty/client/:clientId is deliberately NOT here: it has client-facing AND
// admin consumers, so it stays authenticated-only for every role.
func loyaltyAdminOnlyRoutes() []loyaltyRoute {
	return []loyaltyRoute{
		{"create program", http.MethodPost, "/api/loyalty"},
		{"center programs", http.MethodGet, "/api/loyalty/center/1"},
		{"center settings read", http.MethodGet, "/api/loyalty/center/1/settings"},
		{"center settings update", http.MethodPut, "/api/loyalty/center/1/settings"},
		{"update program", http.MethodPut, "/api/loyalty/1"},
		{"delete program", http.MethodDelete, "/api/loyalty/1"},
		{"toggle program", http.MethodPatch, "/api/loyalty/1/toggle"},
	}
}

// TestLoyaltyAuthz_NoTokenIs401 — every /loyalty route must reject anonymous
// callers with the JWT middleware's response, never reach its handler.
func TestLoyaltyAuthz_NoTokenIs401(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := loyaltyTestRouter(t)
	seedLoyaltyFixtures(t, db)

	for _, rt := range loyaltyAllRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			w := do(r, rt.method, rt.path, "", "")

			assert.Equal(t, http.StatusUnauthorized, w.Code,
				"%s %s must reject anonymous callers", rt.method, rt.path)
			assert.Equal(t, "Authorization header missing", errBody(t, w.Body.String()),
				"handler must not run for an anonymous request")
		})
	}
}

// TestLoyaltyAuthz_AdminRoutesRequireBeautyCenter — the admin operations are
// 403 for client and personnel tokens, and still reachable for beauty_center.
func TestLoyaltyAuthz_AdminRoutesRequireBeautyCenter(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := loyaltyTestRouter(t)
	seedLoyaltyFixtures(t, db)

	for _, rt := range loyaltyAdminOnlyRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			for _, role := range []string{"client", "personnel"} {
				w := do(r, rt.method, rt.path, roleToken(t, 1, 1, role), "")
				assert.Equal(t, http.StatusForbidden, w.Code,
					"%s %s must be 403 for role %s", rt.method, rt.path, role)
				assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
			}

			w := do(r, rt.method, rt.path, roleToken(t, 1, 1, "beauty_center"), "")
			assert.NotEqual(t, http.StatusUnauthorized, w.Code)
			assert.NotEqual(t, http.StatusForbidden, w.Code)
			assert.NotEqual(t, "forbidden", errBody(t, w.Body.String()))
		})
	}
}

// TestLoyaltyAuthz_ClientCardsStayOpenToAuthenticatedUsers — GET
// /loyalty/client/:clientId must keep working for the signed-in client (mobile
// PerfilScreen punch cards) and for the center admin inspecting a client (web
// LoyaltyPage). It is authenticated-only, not role-gated.
func TestLoyaltyAuthz_ClientCardsStayOpenToAuthenticatedUsers(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := loyaltyTestRouter(t)
	seedLoyaltyFixtures(t, db)

	t.Run("authenticated client reads own cards", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/loyalty/client/1", roleToken(t, 1, 0, "client"), "")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"current_steps":3`)
	})

	t.Run("authenticated beauty_center admin reads a client's cards", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/loyalty/client/1", roleToken(t, 7, 1, "beauty_center"), "")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"current_steps":3`)
	})

	t.Run("anonymous client cards request is 401", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/loyalty/client/1", "", "")

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// TestLoyaltyClientCards_CrossClientReadForbidden — IDOR closed. A `client`
// token may only read the cards of the clientID in its own claim; asking for
// another client's :clientId is 403 and must not leak a single card. This
// inverts the previous "known gap" pin.
func TestLoyaltyClientCards_CrossClientReadForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := loyaltyTestRouter(t)
	seedLoyaltyFixtures(t, db)

	// Caller is client 9; cards belong to client 1.
	w := do(r, http.MethodGet, "/api/loyalty/client/1", roleToken(t, 9, 0, "client"), "")

	assert.Equal(t, http.StatusForbidden, w.Code,
		"a client must not read another client's punch cards")
	assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	assert.NotContains(t, w.Body.String(), "current_steps",
		"no card data may leak to the rejected caller")

	// The owner still reads its own cards unchanged.
	w = do(r, http.MethodGet, "/api/loyalty/client/1", roleToken(t, 1, 0, "client"), "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"current_steps":3`)
}

// TestLoyaltyClientCards_PersonnelReadUnchanged pins that the IDOR fix is
// scoped to the `client` role only: personnel (and beauty_center) keep
// inspecting a client's cards, which is a legitimate staff-facing read.
func TestLoyaltyClientCards_PersonnelReadUnchanged(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := loyaltyTestRouter(t)
	seedLoyaltyFixtures(t, db)

	w := do(r, http.MethodGet, "/api/loyalty/client/1", roleToken(t, 7, 1, "personnel"), "")

	assert.Equal(t, http.StatusOK, w.Code, "personnel legitimately inspects a client's cards")
	assert.Contains(t, w.Body.String(), `"current_steps":3`)
}

// TestLoyaltyPrograms_CrossCenterWriteForbidden — RequireRole proves the ROLE,
// not OWNERSHIP. A center-1 admin must not be able to update/delete/toggle a
// center-2 program, and center 2's row must be byte-for-byte untouched after
// the attempts. The legitimate owner still works.
func TestLoyaltyPrograms_CrossCenterWriteForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := loyaltyTestRouter(t)
	seedLoyaltyFixtures(t, db)
	seedSecondCenterWithProgram(t, db)

	const updateBody = `{"required_steps":99,"reward_type":"free","is_active":false}`
	const toggleBody = `{"is_active":false}`

	t.Run("update another center's program is 403", func(t *testing.T) {
		w := do(r, http.MethodPut, "/api/loyalty/2", roleToken(t, 1, 1, "beauty_center"), updateBody)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("toggle another center's program is 403", func(t *testing.T) {
		w := do(r, http.MethodPatch, "/api/loyalty/2/toggle", roleToken(t, 1, 1, "beauty_center"), toggleBody)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("delete another center's program is 403", func(t *testing.T) {
		w := do(r, http.MethodDelete, "/api/loyalty/2", roleToken(t, 1, 1, "beauty_center"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	// The foreign row must survive all three attempts unmodified.
	var p models.LoyaltyProgram
	assert.NoError(t, db.First(&p, 2).Error, "another center's program must not be deleted")
	assert.Equal(t, 5, p.RequiredSteps, "required_steps must not be modified by a foreign admin")
	assert.True(t, p.IsActive, "is_active must not be toggled by a foreign admin")

	// The legitimate owner (center 2) can still manage its program.
	t.Run("owner center updates its program", func(t *testing.T) {
		w := do(r, http.MethodPut, "/api/loyalty/2", roleToken(t, 9, 2, "beauty_center"), `{"required_steps":8}`)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("owner center toggles its program", func(t *testing.T) {
		w := do(r, http.MethodPatch, "/api/loyalty/2/toggle", roleToken(t, 9, 2, "beauty_center"), toggleBody)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("owner center deletes its program", func(t *testing.T) {
		w := do(r, http.MethodDelete, "/api/loyalty/2", roleToken(t, 9, 2, "beauty_center"), "")
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

// TestLoyaltyCenterSettings_CrossCenterForbidden — the :centerId family must
// reject a path value that does not match the caller's centerID claim, and the
// foreign row must stay untouched. GET /center/:otherId, GET/PUT
// /center/:otherId/settings all return 403 for a center admin.
func TestLoyaltyCenterSettings_CrossCenterForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := loyaltyTestRouter(t)
	seedLoyaltyFixtures(t, db)
	seedSecondCenterWithProgram(t, db)

	// center-1 admin probing center 2's resources.
	w := do(r, http.MethodGet, "/api/loyalty/center/2", roleToken(t, 1, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusForbidden, w.Code, "foreign center program list must be 403")
	assert.Equal(t, "forbidden", errBody(t, w.Body.String()))

	w = do(r, http.MethodGet, "/api/loyalty/center/2/settings", roleToken(t, 1, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusForbidden, w.Code, "foreign center settings read must be 403")
	assert.Equal(t, "forbidden", errBody(t, w.Body.String()))

	w = do(r, http.MethodPut, "/api/loyalty/center/2/settings", roleToken(t, 1, 1, "beauty_center"), `{"loyalty_enabled":true}`)
	assert.Equal(t, http.StatusForbidden, w.Code, "foreign center settings write must be 403")
	assert.Equal(t, "forbidden", errBody(t, w.Body.String()))

	// Center 2's loyalty flag must be unchanged.
	var c2 models.BeautyCenter
	assert.NoError(t, db.First(&c2, 2).Error)
	assert.False(t, c2.LoyaltyEnabled, "a foreign admin must not flip another center's loyalty flag")

	// The owner still works end to end.
	w = do(r, http.MethodGet, "/api/loyalty/center/1/settings", roleToken(t, 1, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusOK, w.Code)
	w = do(r, http.MethodPut, "/api/loyalty/center/1/settings", roleToken(t, 1, 1, "beauty_center"), `{"loyalty_enabled":true}`)
	assert.Equal(t, http.StatusOK, w.Code)
	w = do(r, http.MethodGet, "/api/loyalty/center/1", roleToken(t, 1, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestLoyaltyPrograms_CreateScopedToCallerCenter — POST /loyalty must ignore the
// body's beauty_center_id and bind the new program to the caller's centerID
// claim, exactly like reservation Create overrides req.ClientID from the token.
// Before this fix a center-1 admin could POST {"beauty_center_id":3,...} and
// plant a program owned by center 3 — the same IDOR the PUT/DELETE/PATCH family
// closed in the previous pass.
//
// A beauty_center token with no centerID claim cannot own a program, so the
// request is rejected with 403 instead of persisting a zero-center orphan row.
func TestLoyaltyPrograms_CreateScopedToCallerCenter(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := loyaltyTestRouter(t)
	seedLoyaltyFixtures(t, db)
	seedDashboardCenter(t, db, 3, "Foreign Center", "foreign@test.com", true)

	const body = `{"beauty_center_id":3,"service_id":null,"reward_service_id":null,"required_steps":5,"reward_type":"free","discount_value":0}`

	t.Run("body cannot create a program for another center", func(t *testing.T) {
		var before int64
		assert.NoError(t, db.Model(&models.LoyaltyProgram{}).
			Where("beauty_center_id = ?", 1).Count(&before).Error)

		w := do(r, http.MethodPost, "/api/loyalty", roleToken(t, 1, 1, "beauty_center"), body)

		assert.Equal(t, http.StatusCreated, w.Code,
			"the request still succeeds — the row just must belong to the caller")

		var foreign int64
		assert.NoError(t, db.Model(&models.LoyaltyProgram{}).
			Where("beauty_center_id = ?", 3).Count(&foreign).Error)
		assert.Equal(t, int64(0), foreign,
			"a center-1 admin must never create a row owned by center 3")

		var after int64
		assert.NoError(t, db.Model(&models.LoyaltyProgram{}).
			Where("beauty_center_id = ?", 1).Count(&after).Error)
		assert.Equal(t, before+1, after,
			"the program is persisted under the caller's own center instead")
	})

	t.Run("owner center creates its own program with 201", func(t *testing.T) {
		w := do(r, http.MethodPost, "/api/loyalty", roleToken(t, 7, 1, "beauty_center"),
			`{"beauty_center_id":1,"service_id":null,"reward_service_id":null,"required_steps":7,"reward_type":"free","discount_value":0}`)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), `"required_steps":7`)

		var p models.LoyaltyProgram
		assert.NoError(t, db.Where("beauty_center_id = ? AND required_steps = ?", 1, 7).
			First(&p).Error, "the program is persisted under the caller's center")
	})

	t.Run("beauty_center token without a centerID claim is rejected", func(t *testing.T) {
		w := do(r, http.MethodPost, "/api/loyalty", roleToken(t, 7, 0, "beauty_center"),
			`{"beauty_center_id":1,"service_id":null,"reward_service_id":null,"required_steps":5,"reward_type":"free","discount_value":0}`)

		assert.Equal(t, http.StatusForbidden, w.Code,
			"a center admin with no centerID claim must not create an orphan program")
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))

		var orphan int64
		assert.NoError(t, db.Model(&models.LoyaltyProgram{}).
			Where("beauty_center_id = ?", 0).Count(&orphan).Error)
		assert.Equal(t, int64(0), orphan, "no zero-center program may be created")
	})
}
