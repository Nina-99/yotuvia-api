package tests

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
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

// platformTestRouter mounts the platform surface (and the centre-scoped
// payment/token surfaces it must not weaken) exactly as router.go does, over the
// shared SQLite schema. UPLOAD_DIR is redirected to a temp dir so the real QR
// upload path runs without polluting the repository.
func platformTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("UPLOAD_DIR", t.TempDir())
	db := setupRouterAuthzDB(t)
	r := gin.New()
	r.Use(gin.Recovery())
	rg := r.Group("/api")
	validate := validator.New()
	router.PaymentConfigRoutes(rg, db, validate)
	router.TokenRoutes(rg, db, validate)
	router.PlatformRoutes(rg, db, validate)
	return r, db
}

type platformRoute struct {
	name   string
	method string
	path   string
	body   string
}

// platformAllRoutes is the whole /admin/platform surface.
func platformAllRoutes() []platformRoute {
	return []platformRoute{
		{"global topup queue", http.MethodGet, "/api/admin/platform/tokens/topups", ""},
		{"platform process topup", http.MethodPut, "/api/admin/platform/tokens/topups/1", `{"status":"approved"}`},
		{"center picker", http.MethodGet, "/api/admin/platform/centers", ""},
		{"platform verify center", http.MethodPut, "/api/admin/platform/centers/1/verify", ""},
		{"platform payment read", http.MethodGet, "/api/admin/platform/centers/1/payment", ""},
		{"platform payment update", http.MethodPut, "/api/admin/platform/centers/1/payment", `{"payment_phone":"x"}`},
		{"platform payment qr", http.MethodPost, "/api/admin/platform/centers/1/payment/qr", ""},
	}
}

// seedPlatformFixtures creates centers 1 and 2 with distinct payment configs and
// one top-up each, so cross-centre probes have real foreign data to attack.
func seedPlatformFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedDashboardCenter(t, db, 1, "Platform Center One", "p1@test.com", true)
	seedDashboardCenter(t, db, 2, "Platform Center Two", "p2@test.com", true)

	require.NoError(t, db.Create(&models.PaymentConfig{
		BeautyCenterID: 1,
		PaymentQRURL:   "qr-platform-1",
		PaymentPhone:   "111",
		TiersJSON:      `{"USD":[{"min":10,"bonusPct":5}]}`,
	}).Error)
	require.NoError(t, db.Create(&models.PaymentConfig{
		BeautyCenterID: 2,
		PaymentQRURL:   "qr-platform-2",
		PaymentPhone:   "222",
		TiersJSON:      `{"USD":[{"min":20,"bonusPct":10}]}`,
	}).Error)

	require.NoError(t, db.Create(&models.TokenTopUp{
		ID: 1, BeautyCenterID: 1, AmountBs: 100, TokensReceived: 100000,
		BonusTokens: 0, ReceiptURL: "receipt-1", Status: "pending",
	}).Error)
	require.NoError(t, db.Create(&models.TokenTopUp{
		ID: 2, BeautyCenterID: 2, AmountBs: 200, TokensReceived: 200000,
		BonusTokens: 20000, ReceiptURL: "receipt-2", Status: "pending",
	}).Error)
}

// TestPlatformAuthz_NoTokenIs401 — every platform route rejects anonymous
// callers with the JWT middleware's response, never reaching its handler.
func TestPlatformAuthz_NoTokenIs401(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := platformTestRouter(t)
	seedPlatformFixtures(t, db)

	for _, rt := range platformAllRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			w := do(r, rt.method, rt.path, "", rt.body)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Equal(t, "Authorization header missing", errBody(t, w.Body.String()))
		})
	}
}

// TestPlatformAuthz_OtherRolesAreForbidden — client, beauty_center and
// personnel tokens are 403 on EVERY platform route. The platform surface is not
// reachable by any pre-existing role.
func TestPlatformAuthz_OtherRolesAreForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := platformTestRouter(t)
	seedPlatformFixtures(t, db)

	for _, rt := range platformAllRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			for _, role := range []string{"client", "beauty_center", "personnel"} {
				w := do(r, rt.method, rt.path, roleToken(t, 1, 1, role), rt.body)
				assert.Equal(t, http.StatusForbidden, w.Code,
					"%s %s must be 403 for role %s", rt.method, rt.path, role)
				assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
			}
		})
	}
}

// TestPlatformAuthz_SuperAdminReachesHandlers — a super_admin token (centre
// claim 0) passes the gate and runs the handler: every route answers something
// other than 401/403.
func TestPlatformAuthz_SuperAdminReachesHandlers(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := platformTestRouter(t)
	seedPlatformFixtures(t, db)

	for _, rt := range platformAllRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			// Centre claim MUST be 0: the platform admin belongs to no centre.
			w := do(r, rt.method, rt.path, roleToken(t, 900, 0, "super_admin"), rt.body)
			assert.NotEqual(t, http.StatusUnauthorized, w.Code)
			assert.NotEqual(t, http.StatusForbidden, w.Code)
			assert.NotEqual(t, "forbidden", errBody(t, w.Body.String()))
		})
	}
}

// TestPlatformAuthz_SuperAdminIsStillBlockedFromCentreScopedRoutes — the new
// role gains NO access to the existing centre-scoped surfaces. With no centre
// claim, `RequireRole("beauty_center")` rejects it exactly like any other role.
func TestPlatformAuthz_SuperAdminIsStillBlockedFromCentreScopedRoutes(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := platformTestRouter(t)
	seedPlatformFixtures(t, db)

	for _, rt := range []platformRoute{
		{"centre payment update", http.MethodPut, "/api/admin/config/payment", `{"payment_phone":"hijack"}`},
		{"centre topup processing", http.MethodPut, "/api/admin/tokens/topups/1", `{"status":"approved"}`},
	} {
		t.Run(rt.name, func(t *testing.T) {
			w := do(r, rt.method, rt.path, roleToken(t, 900, 0, "super_admin"), rt.body)
			assert.Equal(t, http.StatusForbidden, w.Code,
				"a super_admin must not reach a centre-scoped route")
			assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
		})
	}

	// Centre 1's config and top-up must be untouched by those attempts.
	var cfg models.PaymentConfig
	require.NoError(t, db.Where("beauty_center_id = ?", 1).First(&cfg).Error)
	assert.Equal(t, "111", cfg.PaymentPhone)
	var topup models.TokenTopUp
	require.NoError(t, db.First(&topup, 1).Error)
	assert.Equal(t, "pending", topup.Status)
}

type platformTopUpListResp struct {
	TopUps []struct {
		ID             uint   `json:"id"`
		BeautyCenterID uint   `json:"beauty_center_id"`
		Status         string `json:"status"`
	} `json:"topups"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
}

// TestPlatform_GlobalQueueSpansCentresAndFiltersHonestly — the queue lists
// top-ups from MORE THAN ONE centre, and both filters really narrow the query:
// beauty_center_id picks one centre, status picks one state.
func TestPlatform_GlobalQueueSpansCentresAndFiltersHonestly(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := platformTestRouter(t)
	seedPlatformFixtures(t, db)
	// A third, already-processed top-up so the status filter has something to
	// exclude while the centre filter has something to keep.
	require.NoError(t, db.Create(&models.TokenTopUp{
		ID: 3, BeautyCenterID: 2, AmountBs: 50, TokensReceived: 50000,
		BonusTokens: 0, ReceiptURL: "receipt-3", Status: "approved",
	}).Error)

	token := roleToken(t, 900, 0, "super_admin")

	w := do(r, http.MethodGet, "/api/admin/platform/tokens/topups", token, "")
	require.Equal(t, http.StatusOK, w.Code)
	var all platformTopUpListResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &all))
	require.Equal(t, int64(3), all.Total, "every centre's top-up is listed")
	require.Len(t, all.TopUps, 3)
	centres := map[uint]bool{}
	for _, tu := range all.TopUps {
		centres[tu.BeautyCenterID] = true
	}
	assert.True(t, centres[1] && centres[2],
		"the global queue must span centre 1 and centre 2, got %v", centres)

	w = do(r, http.MethodGet, "/api/admin/platform/tokens/topups?beauty_center_id=2", token, "")
	require.Equal(t, http.StatusOK, w.Code)
	var byCentre platformTopUpListResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &byCentre))
	require.Equal(t, int64(2), byCentre.Total)
	require.Len(t, byCentre.TopUps, 2)
	for _, tu := range byCentre.TopUps {
		assert.Equal(t, uint(2), tu.BeautyCenterID, "the centre filter really narrows")
	}

	w = do(r, http.MethodGet, "/api/admin/platform/tokens/topups?status=pending", token, "")
	require.Equal(t, http.StatusOK, w.Code)
	var pending platformTopUpListResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pending))
	require.Equal(t, int64(2), pending.Total)
	for _, tu := range pending.TopUps {
		assert.Equal(t, "pending", tu.Status, "the status filter really narrows")
	}

	w = do(r, http.MethodGet, "/api/admin/platform/tokens/topups?status=approved&beauty_center_id=2", token, "")
	require.Equal(t, http.StatusOK, w.Code)
	var combined platformTopUpListResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &combined))
	require.Equal(t, int64(1), combined.Total)
	assert.Equal(t, uint(3), combined.TopUps[0].ID)

	w = do(r, http.MethodGet, "/api/admin/platform/tokens/topups?beauty_center_id=abc", token, "")
	assert.Equal(t, http.StatusBadRequest, w.Code, "a malformed centre filter is a client error")
}

// TestPlatform_ApproveTopUpOfAnotherCentre — the whole point of the role: a
// top-up owned by centre 2, which a centre-1 admin cannot process, is approved
// by the platform admin and credits CENTRE 2's account. The already-processed
// guard is reused unchanged.
func TestPlatform_ApproveTopUpOfAnotherCentre(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := platformTestRouter(t)
	seedPlatformFixtures(t, db)
	require.NoError(t, db.Create(&models.TokenAccount{BeautyCenterID: 2, Balance: 0}).Error)

	// Sanity: a centre-1 admin cannot process centre 2's top-up through the
	// centre-scoped route (the pre-existing ownership contract).
	w := do(r, http.MethodPut, "/api/admin/tokens/topups/2",
		roleToken(t, 50, 1, "beauty_center"), `{"status":"approved"}`)
	assert.Equal(t, http.StatusForbidden, w.Code,
		"a centre-1 admin must not process centre 2's top-up")

	// The platform admin can.
	w = do(r, http.MethodPut, "/api/admin/platform/tokens/topups/2",
		roleToken(t, 900, 0, "super_admin"), `{"status":"approved"}`)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var topup models.TokenTopUp
	require.NoError(t, db.First(&topup, 2).Error)
	assert.Equal(t, "approved", topup.Status)
	require.NotNil(t, topup.ValidatedBy)
	assert.Equal(t, uint(900), *topup.ValidatedBy, "the platform admin's own id is recorded")

	var account models.TokenAccount
	require.NoError(t, db.Where("beauty_center_id = ?", 2).First(&account).Error)
	assert.Equal(t, int64(220000), account.Balance,
		"tokens_received + bonus_tokens credited to the OWNING centre")

	// Already-processed behaviour is the existing one, not a new contract.
	w = do(r, http.MethodPut, "/api/admin/platform/tokens/topups/2",
		roleToken(t, 900, 0, "super_admin"), `{"status":"approved"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "top-up already processed", errBody(t, w.Body.String()))
}

// TestPlatform_RejectTopUpOfAnotherCentreLeavesBalanceUntouched — triangulation
// for the decision path: `rejected` never credits the centre.
func TestPlatform_RejectTopUpOfAnotherCentreLeavesBalanceUntouched(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := platformTestRouter(t)
	seedPlatformFixtures(t, db)
	require.NoError(t, db.Create(&models.TokenAccount{BeautyCenterID: 2, Balance: 0}).Error)

	w := do(r, http.MethodPut, "/api/admin/platform/tokens/topups/2",
		roleToken(t, 900, 0, "super_admin"), `{"status":"rejected"}`)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var topup models.TokenTopUp
	require.NoError(t, db.First(&topup, 2).Error)
	assert.Equal(t, "rejected", topup.Status)

	var account models.TokenAccount
	require.NoError(t, db.Where("beauty_center_id = ?", 2).First(&account).Error)
	assert.Zero(t, account.Balance, "a rejected top-up credits nothing")
}

type platformPaymentResp struct {
	BeautyCenterID uint   `json:"beauty_center_id"`
	PaymentQRURL   string `json:"payment_qr_url"`
	PaymentPhone   string `json:"payment_phone"`
}

// TestPlatform_PaymentRoutesOperateOnTheChosenCentre — read and write are keyed
// by the path centre, never by a claim: selecting centre 2 must return centre
// 2's config and must never leak centre 1's.
func TestPlatform_PaymentRoutesOperateOnTheChosenCentre(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := platformTestRouter(t)
	seedPlatformFixtures(t, db)

	token := roleToken(t, 900, 0, "super_admin")

	w := do(r, http.MethodGet, "/api/admin/platform/centers/2/payment", token, "")
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	var two platformPaymentResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &two))
	assert.Equal(t, uint(2), two.BeautyCenterID)
	assert.Equal(t, "qr-platform-2", two.PaymentQRURL)
	assert.Equal(t, "222", two.PaymentPhone)
	assert.NotContains(t, w.Body.String(), "qr-platform-1")

	w = do(r, http.MethodGet, "/api/admin/platform/centers/1/payment", token, "")
	require.Equal(t, http.StatusOK, w.Code)
	var one platformPaymentResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &one))
	assert.Equal(t, "qr-platform-1", one.PaymentQRURL)

	// Update the chosen centre only.
	w = do(r, http.MethodPut, "/api/admin/platform/centers/2/payment", token, `{"payment_phone":"222-platform"}`)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var cfgOne, cfgTwo models.PaymentConfig
	require.NoError(t, db.Where("beauty_center_id = ?", 1).First(&cfgOne).Error)
	require.NoError(t, db.Where("beauty_center_id = ?", 2).First(&cfgTwo).Error)
	assert.Equal(t, "222-platform", cfgTwo.PaymentPhone, "the chosen centre is written")
	assert.Equal(t, "111", cfgOne.PaymentPhone, "the other centre is untouched")
	assert.Equal(t, "qr-platform-2", cfgTwo.PaymentQRURL, "a phone-only write keeps the QR")

	// An unknown centre is a 404, never a phantom config row.
	w = do(r, http.MethodGet, "/api/admin/platform/centers/999/payment", token, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	var phantom int64
	require.NoError(t, db.Model(&models.PaymentConfig{}).
		Where("beauty_center_id = ?", 999).Count(&phantom).Error)
	assert.Zero(t, phantom, "a platform read must not create a phantom config row")
}

// TestPlatform_UploadQROperatesOnTheChosenCentre — the QR upload reuses the
// production upload + payment service, keyed by the path centre.
func TestPlatform_UploadQROperatesOnTheChosenCentre(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := platformTestRouter(t)
	seedPlatformFixtures(t, db)

	token := roleToken(t, 900, 0, "super_admin")

	w := doMultipartPNG(t, r, "/api/admin/platform/centers/2/payment/qr", token)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var uploaded struct {
		URL string `json:"url"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &uploaded))
	require.NotEmpty(t, uploaded.URL, "the upload returns the public URL")

	var cfgOne, cfgTwo models.PaymentConfig
	require.NoError(t, db.Where("beauty_center_id = ?", 1).First(&cfgOne).Error)
	require.NoError(t, db.Where("beauty_center_id = ?", 2).First(&cfgTwo).Error)
	assert.Equal(t, uploaded.URL, cfgTwo.PaymentQRURL, "the chosen centre's QR is replaced")
	assert.Equal(t, "qr-platform-1", cfgOne.PaymentQRURL, "the other centre's QR is untouched")
}

// TestPlatform_CenterPickerListsEveryCentre — the picker returns id + name for
// the centres a platform admin oversees.
func TestPlatform_CenterPickerListsEveryCentre(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := platformTestRouter(t)
	seedPlatformFixtures(t, db)

	w := do(r, http.MethodGet, "/api/admin/platform/centers", roleToken(t, 900, 0, "super_admin"), "")
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp struct {
		Centers []struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		} `json:"centers"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Centers, 2, "both centres are offered, got %+v", resp.Centers)
	byID := map[uint]string{}
	for _, c := range resp.Centers {
		byID[c.ID] = c.Name
	}
	assert.Equal(t, "Platform Center One", byID[1])
	assert.Equal(t, "Platform Center Two", byID[2])
}

// doMultipartPNG posts a one-part multipart body carrying REAL PNG bytes so the
// real upload path runs. The payload must be a genuine image: since the uploader
// derives the stored extension from the sniffed bytes (not from the client's
// Content-Type header or filename), a fake body is correctly rejected as
// non-image.
func doMultipartPNG(t *testing.T, r *gin.Engine, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="qr.png"`)
	header.Set("Content-Type", "image/png")
	part, err := mw.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write(pngBytes)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req, err := http.NewRequest(http.MethodPost, path, &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
