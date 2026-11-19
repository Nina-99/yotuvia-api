package tests

// Regression coverage for the 2026-11-14 audit's Block 1: three IDOR findings
// that let an authenticated account write into ANOTHER centre's data.
//
// The attacks, verified with a live PoC before the fix:
//   - DELETE /api/services/1 as a `client` token  -> 200 (catalog destroyed)
//   - DELETE /api/category/1 as a `client` token  -> 200
//   - PUT /api/personnel/1 as another centre's admin -> 200 AND password reset
//
// Every case below asserts the closed behaviour: the caller's ROLE is proven by
// the router and the ROW OWNERSHIP by the service, and the body's
// `beauty_center_id` is never trusted.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type tenantFixtures struct {
	serviceID   uint
	categoryID  uint
	personnelID uint
}

// tenantRouter mounts the three catalog surfaces over the shared SQLite schema
// and seeds one service, one category and one staff member, all owned by centre
// 1 (the victim). The attacker owns centre 2.
func tenantRouter(t *testing.T) (*gin.Engine, *gorm.DB, tenantFixtures) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	validate := validator.New()

	svc := models.Service{
		BeautyCenterID: 1, CategoryID: 1, Name: "Corte", Price: 1000,
		Duration: 30, IsActive: true,
	}
	require.NoError(t, db.Create(&svc).Error)
	cat := models.Category{BeautyCenterID: 1, Name: "Peluqueria"}
	require.NoError(t, db.Create(&cat).Error)
	staff := models.Personnel{
		BeautyCenterID: 1, FirstName: "Ana", LastName: "Victima",
		Email: "ana@victim.com", IsActive: true,
	}
	hash, err := utils.HashPassword("original-pass")
	require.NoError(t, err)
	staff.Password = hash
	require.NoError(t, db.Create(&staff).Error)

	r := gin.New()
	rg := r.Group("/api")
	router.ServiceRoutes(rg, db, validate)
	router.CategoryRoutes(rg, db, validate)
	router.PersonnelRoutes(rg, db, validate)

	return r, db, tenantFixtures{serviceID: svc.ID, categoryID: cat.ID, personnelID: staff.ID}
}

// TestServiceWrites_AreCentreScoped
func TestServiceWrites_AreCentreScoped(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db, f := tenantRouter(t)

	clientTok := roleToken(t, 9, 0, "client")
	attackerTok := roleToken(t, 2, 2, "beauty_center") // centre 2
	victimTok := roleToken(t, 1, 1, "beauty_center")   // centre 1

	t.Run("client cannot delete a centre's service", func(t *testing.T) {
		w := do(r, http.MethodDelete, "/api/services/1", clientTok, "")
		assert.Equal(t, http.StatusForbidden, w.Code)

		var count int64
		require.NoError(t, db.Model(&models.Service{}).Where("id = ?", f.serviceID).Count(&count).Error)
		assert.Equal(t, int64(1), count, "the service must survive")
	})

	t.Run("client cannot update a centre's service", func(t *testing.T) {
		w := do(r, http.MethodPut, "/api/services/1", clientTok, `{"name":"HACKED","price":1}`)
		assert.Equal(t, http.StatusForbidden, w.Code)

		w2 := do(r, http.MethodPost, "/api/services", clientTok,
			`{"beauty_center_id":1,"category_id":1,"name":"Free","description":"x","price":10,"duration":5}`)
		assert.Equal(t, http.StatusForbidden, w2.Code)
	})

	t.Run("another centre cannot update or delete the service", func(t *testing.T) {
		w := do(r, http.MethodPut, "/api/services/1", attackerTok, `{"name":"HACKED"}`)
		assert.Equal(t, http.StatusForbidden, w.Code)

		w2 := do(r, http.MethodDelete, "/api/services/1", attackerTok, "")
		assert.Equal(t, http.StatusForbidden, w2.Code)

		var stored models.Service
		require.NoError(t, db.First(&stored, f.serviceID).Error)
		assert.Equal(t, "Corte", stored.Name)
	})

	t.Run("owner can update its own service", func(t *testing.T) {
		w := do(r, http.MethodPut, "/api/services/1", victimTok, `{"name":"Corte premium"}`)
		assert.Equal(t, http.StatusOK, w.Code)

		var stored models.Service
		require.NoError(t, db.First(&stored, f.serviceID).Error)
		assert.Equal(t, "Corte premium", stored.Name)
	})

	t.Run("body beauty_center_id cannot move a service to another centre", func(t *testing.T) {
		w := do(r, http.MethodPut, "/api/services/1", victimTok, `{"beauty_center_id":2,"name":"Mudado"}`)
		assert.Equal(t, http.StatusOK, w.Code)

		var stored models.Service
		require.NoError(t, db.First(&stored, f.serviceID).Error)
		assert.Equal(t, uint(1), stored.BeautyCenterID, "a service can never be re-parented")
	})

	t.Run("create binds the owner to the claim, not the body", func(t *testing.T) {
		w := do(r, http.MethodPost, "/api/services", attackerTok,
			`{"beauty_center_id":1,"category_id":1,"name":"Inyectado","description":"x","price":10,"duration":5}`)
		// Category 1 belongs to centre 1, so a centre-2 admin cannot use it.
		assert.Equal(t, http.StatusForbidden, w.Code)

		var leaked int64
		require.NoError(t, db.Model(&models.Service{}).Where("name = ?", "Inyectado").Count(&leaked).Error)
		assert.Zero(t, leaked, "no row may be created in the victim's centre")
	})
}

// TestCategoryWrites_AreCentreScoped
func TestCategoryWrites_AreCentreScoped(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db, f := tenantRouter(t)

	clientTok := roleToken(t, 9, 0, "client")
	attackerTok := roleToken(t, 2, 2, "beauty_center")
	victimTok := roleToken(t, 1, 1, "beauty_center")

	w := do(r, http.MethodDelete, "/api/category/1", clientTok, "")
	assert.Equal(t, http.StatusForbidden, w.Code)

	w = do(r, http.MethodDelete, "/api/category/1", attackerTok, "")
	assert.Equal(t, http.StatusForbidden, w.Code)

	var count int64
	require.NoError(t, db.Model(&models.Category{}).Where("id = ?", f.categoryID).Count(&count).Error)
	assert.Equal(t, int64(1), count, "the category must survive both attempts")

	// Owner can rename it, and cannot smuggle it into another centre.
	w = do(r, http.MethodPut, "/api/category/1", victimTok, `{"beauty_center_id":2,"name":"Peluqueria v2"}`)
	assert.Equal(t, http.StatusOK, w.Code)

	var stored models.Category
	require.NoError(t, db.First(&stored, f.categoryID).Error)
	assert.Equal(t, "Peluqueria v2", stored.Name)
	assert.Equal(t, uint(1), stored.BeautyCenterID)
}

// TestPersonnelWrites_AreCentreScoped — the account-takeover primitive.
func TestPersonnelWrites_AreCentreScoped(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db, f := tenantRouter(t)

	attackerTok := roleToken(t, 2, 2, "beauty_center") // centre 2
	victimTok := roleToken(t, 1, 1, "beauty_center")   // centre 1

	t.Run("another centre cannot edit the staff member", func(t *testing.T) {
		w := do(r, http.MethodPut, "/api/personnel/1", attackerTok,
			`{"first_name":"Owned","phone":"000"}`)
		assert.Equal(t, http.StatusForbidden, w.Code)

		var stored models.Personnel
		require.NoError(t, db.First(&stored, f.personnelID).Error)
		assert.Equal(t, "Ana", stored.FirstName)
	})

	t.Run("another centre cannot delete the staff member", func(t *testing.T) {
		w := do(r, http.MethodDelete, "/api/personnel/1", attackerTok, "")
		assert.Equal(t, http.StatusForbidden, w.Code)

		var count int64
		require.NoError(t, db.Model(&models.Personnel{}).Where("id = ?", f.personnelID).Count(&count).Error)
		assert.Equal(t, int64(1), count)
	})

	t.Run("a password in the admin update is ignored", func(t *testing.T) {
		var before models.Personnel
		require.NoError(t, db.First(&before, f.personnelID).Error)

		w := do(r, http.MethodPut, "/api/personnel/1", victimTok,
			`{"first_name":"Ana","password":"Pwned123!","is_active":true}`)
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

		var after models.Personnel
		require.NoError(t, db.First(&after, f.personnelID).Error)
		assert.Equal(t, before.Password, after.Password,
			"the admin edit path must never rewrite a staff password")
		assert.False(t, utils.VerifyPassword("Pwned123!", after.Password),
			"the injected password must not authenticate")
	})

	t.Run("owner can edit its own staff", func(t *testing.T) {
		w := do(r, http.MethodPut, "/api/personnel/1", victimTok, `{"first_name":"Ana Maria","is_active":true}`)
		assert.Equal(t, http.StatusOK, w.Code)

		var res struct {
			FirstName string `json:"first_name"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		assert.Equal(t, "Ana Maria", res.FirstName)
	})
}
