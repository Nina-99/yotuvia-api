package tests

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

// paymentConfigTestRouter mounts PaymentConfigRoutes the way router.go does,
// over the shared SQLite schema. Recovery keeps a handler panic from crashing
// the binary so it surfaces as a 500 assertion failure instead.
func paymentConfigTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	r := gin.New()
	r.Use(gin.Recovery())
	router.PaymentConfigRoutes(r.Group("/api"), db, validator.New())
	return r, db
}

// seedPaymentConfigFixtures creates three centres: 1 and 2 each own a distinct
// payment config (so a cross-centre probe has real foreign data to attack) and
// 3 exists with NO config row (so the "never fall back" probe has a centre whose
// config must resolve empty, never to another centre's QR).
func seedPaymentConfigFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedDashboardCenter(t, db, 1, "Config Center One", "cfg1@test.com", true)
	seedDashboardCenter(t, db, 2, "Config Center Two", "cfg2@test.com", true)
	seedDashboardCenter(t, db, 3, "Config Center Three", "cfg3@test.com", true)

	require.NoError(t, db.Create(&models.PaymentConfig{
		BeautyCenterID: 1,
		PaymentQRURL:   "qr-center-1",
		PaymentPhone:   "111",
		TiersJSON:      `{"USD":[{"min":10,"bonusPct":5}]}`,
	}).Error)
	require.NoError(t, db.Create(&models.PaymentConfig{
		BeautyCenterID: 2,
		PaymentQRURL:   "qr-center-2",
		PaymentPhone:   "222",
		TiersJSON:      `{"USD":[{"min":20,"bonusPct":10}]}`,
	}).Error)
}

type publicPaymentConfigResp struct {
	BeautyCenterID uint   `json:"beauty_center_id"`
	PaymentQRURL   string `json:"payment_qr_url"`
	PaymentPhone   string `json:"payment_phone"`
	Tiers          map[string][]struct {
		Min      float64 `json:"min"`
		BonusPct float64 `json:"bonusPct"`
	} `json:"tiers"`
}

// TestPaymentConfigAuthz_NoTokenIs401 — the admin payment routes keep rejecting
// anonymous callers with the JWT middleware's response.
func TestPaymentConfigAuthz_NoTokenIs401(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := paymentConfigTestRouter(t)
	seedPaymentConfigFixtures(t, db)

	for _, rt := range []struct{ method, path string }{
		{http.MethodPut, "/api/admin/config/payment"},
		{http.MethodPost, "/api/admin/config/payment/qr"},
	} {
		w := do(r, rt.method, rt.path, "", `{"payment_phone":"x"}`)
		assert.Equal(t, http.StatusUnauthorized, w.Code,
			"%s %s must reject anonymous callers", rt.method, rt.path)
		assert.Equal(t, "Authorization header missing", errBody(t, w.Body.String()))
	}
}

// TestPaymentConfigAuthz_CenterAdminWritesOnlyOwnCenter — the update is scoped
// to the caller's centreID claim. A centre-1 admin updating its phone must not
// touch centre 2's row, and vice versa. This is the fix for the hijack where any
// beauty_center admin could overwrite the platform-wide singleton.
func TestPaymentConfigAuthz_CenterAdminWritesOnlyOwnCenter(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := paymentConfigTestRouter(t)
	seedPaymentConfigFixtures(t, db)

	w := do(r, http.MethodPut, "/api/admin/config/payment",
		roleToken(t, 7, 1, "beauty_center"), `{"payment_phone":"111-updated"}`)
	require.Equal(t, http.StatusOK, w.Code)

	w = do(r, http.MethodPut, "/api/admin/config/payment",
		roleToken(t, 9, 2, "beauty_center"), `{"payment_phone":"222-updated"}`)
	require.Equal(t, http.StatusOK, w.Code)

	var one, two models.PaymentConfig
	require.NoError(t, db.Where("beauty_center_id = ?", 1).First(&one).Error)
	require.NoError(t, db.Where("beauty_center_id = ?", 2).First(&two).Error)

	assert.Equal(t, "111-updated", one.PaymentPhone,
		"centre 1 admin must write its own row")
	assert.Equal(t, "222-updated", two.PaymentPhone,
		"centre 2 admin must write its own row")
	assert.Equal(t, "qr-center-1", one.PaymentQRURL,
		"centre 1's QR must be untouched by its own phone-only write")
	assert.Equal(t, "qr-center-2", two.PaymentQRURL,
		"centre 2's QR must be untouched by centre 1's write")
}

// TestPaymentConfigAuthz_NoCenterClaimIsForbidden — a beauty_center token with
// no centerID claim (0) cannot own a config row, so both admin writes are
// rejected with 403 instead of writing an unscoped/global row.
func TestPaymentConfigAuthz_NoCenterClaimIsForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := paymentConfigTestRouter(t)
	seedPaymentConfigFixtures(t, db)

	w := do(r, http.MethodPut, "/api/admin/config/payment",
		roleToken(t, 7, 0, "beauty_center"), `{"payment_phone":"hijack"}`)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "forbidden", errBody(t, w.Body.String()))

	w = do(r, http.MethodPost, "/api/admin/config/payment/qr",
		roleToken(t, 7, 0, "beauty_center"), "")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "forbidden", errBody(t, w.Body.String()))

	// No zero-centre row and no cross-centre write may have happened.
	var legacy int64
	assert.NoError(t, db.Model(&models.PaymentConfig{}).
		Where("beauty_center_id = ?", 0).Count(&legacy).Error)
	assert.Equal(t, int64(0), legacy, "a claimless admin must not create a row")

	var one models.PaymentConfig
	require.NoError(t, db.Where("beauty_center_id = ?", 1).First(&one).Error)
	assert.Equal(t, "111", one.PaymentPhone, "centre 1 must be untouched")
}

// TestPaymentConfigPublic_ResolvesRequestedCenter — GET /config/payment must
// return the config of the centre named by `beauty_center_id`.
func TestPaymentConfigPublic_ResolvesRequestedCenter(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := paymentConfigTestRouter(t)
	seedPaymentConfigFixtures(t, db)

	w := do(r, http.MethodGet, "/api/config/payment?beauty_center_id=1", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	var one publicPaymentConfigResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &one))
	assert.Equal(t, uint(1), one.BeautyCenterID)
	assert.Equal(t, "qr-center-1", one.PaymentQRURL)
	assert.Equal(t, "111", one.PaymentPhone)

	w = do(r, http.MethodGet, "/api/config/payment?beauty_center_id=2", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	var two publicPaymentConfigResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &two))
	assert.Equal(t, uint(2), two.BeautyCenterID)
	assert.Equal(t, "qr-center-2", two.PaymentQRURL, "the requested centre's QR is served")
	assert.Equal(t, "222", two.PaymentPhone)
}

// TestPaymentConfigPublic_MissingOrInvalidCenterIsRejected — a missing or
// malformed `beauty_center_id` is a client error, never a silent fallback to a
// different centre's QR.
func TestPaymentConfigPublic_MissingOrInvalidCenterIsRejected(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := paymentConfigTestRouter(t)
	seedPaymentConfigFixtures(t, db)

	for _, path := range []string{
		"/api/config/payment",
		"/api/config/payment?beauty_center_id=abc",
		"/api/config/payment?beauty_center_id=0",
		"/api/config/payment?beauty_center_id=-1",
		"/api/config/tiers",
		"/api/config/tiers?beauty_center_id=abc",
	} {
		w := do(r, http.MethodGet, path, "", "")
		assert.Equal(t, http.StatusBadRequest, w.Code,
			"%s must be rejected with 400, never serve another centre's QR", path)
		assert.NotContains(t, w.Body.String(), "qr-center-1",
			"%s must not leak another centre's config", path)
	}
}

// TestPaymentConfigPublic_NeverFallsBackToAnotherCenter — a centre that exists
// but has no config row (or an unconfigured one) must resolve to its own EMPTY
// config, never to another centre's QR.
func TestPaymentConfigPublic_NeverFallsBackToAnotherCenter(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := paymentConfigTestRouter(t)
	seedPaymentConfigFixtures(t, db)

	w := do(r, http.MethodGet, "/api/config/payment?beauty_center_id=3", "", "")
	require.Equal(t, http.StatusOK, w.Code)

	var three publicPaymentConfigResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &three))
	assert.Equal(t, uint(3), three.BeautyCenterID)
	assert.Empty(t, three.PaymentQRURL, "an unconfigured centre serves no QR, not another's")
	assert.NotContains(t, w.Body.String(), "qr-center-1")
	assert.NotContains(t, w.Body.String(), "qr-center-2")
}

// TestPaymentConfigPublic_TiersResolveRequestedCenter — the public tier read is
// centre-scoped too: centre 2's ladder is 20/10, centre 1's is 10/5.
func TestPaymentConfigPublic_TiersResolveRequestedCenter(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := paymentConfigTestRouter(t)
	seedPaymentConfigFixtures(t, db)

	w := do(r, http.MethodGet, "/api/config/tiers?beauty_center_id=2&currency=USD", "", "")
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Tiers []struct {
			Min      float64 `json:"min"`
			BonusPct float64 `json:"bonusPct"`
		} `json:"tiers"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Tiers, 1)
	assert.Equal(t, float64(20), resp.Tiers[0].Min, "centre 2's own ladder is served")
	assert.Equal(t, float64(10), resp.Tiers[0].BonusPct)
}
