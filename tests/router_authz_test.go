package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/router"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func ptrString(s string) *string { return &s }

// setupRouterAuthzDB migrates the same model set configs/db.config.go does, so
// allowed-role probes can actually reach their handlers on an empty database.
func setupRouterAuthzDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	// :memory: databases are per-connection — a pool with >1 connection gives
	// transactions an EMPTY database (feat-token-economy-config D4 found this
	// the hard way). Pin every statement to the single shared connection.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	modelsToMigrate := []interface{}{
		&models.AuthAccount{}, &models.BeautyCenter{}, &models.BeautyCenterImage{},
		&models.Category{}, &models.TokenAccount{}, &models.TokenTopUp{},
		&models.TokenTransaction{}, &models.TokenMission{}, &models.TokenMissionProgress{},
		&models.Referral{}, &models.Client{}, &models.CustomerPunchCard{},
		&models.Favorite{}, &models.FeedPost{}, &models.LoyaltyProgram{},
		&models.Personnel{}, &models.PersonnelImage{}, &models.PersonnelService{},
		&models.PostLike{}, &models.PunchHistory{}, &models.Reservation{},
		&models.ReservationService{}, &models.Review{}, &models.Service{},
		&models.SocialMedia{}, &models.WorkingHour{}, &models.PersonnelInvitation{}, &models.Notification{}, &models.PushDevice{},
		&models.MarketConfig{}, &models.VideoComment{},
	}
	for _, m := range modelsToMigrate {
		if err := db.AutoMigrate(m); err != nil {
			t.Fatalf("failed to migrate %T: %v", m, err)
		}
	}
	return db
}

// authzTestRouter mounts the routers exactly as router.go does, including
// TokenRoutes (design D5: the topup routeGate rows need the /admin/tokens
// group mounted with its RequireRole("beauty_center") gate).
func authzTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	validate := validator.New()

	r := gin.New()
	rg := r.Group("/api")
	router.ReservationRoutes(rg, db, validate)
	router.ClientRoutes(rg, db, validate)
	router.PersonnelRoutes(rg, db, validate)
	router.NotificationRoutes(rg, db)
	router.TokenRoutes(rg, db, validate)
	return r, db
}

// roleToken issues an HS256 token for the given role via the production path.
func roleToken(t *testing.T, userID, centerID uint, role string) string {
	t.Helper()
	token, err := GenerateTestJWT(userID, centerID, role)
	if err != nil {
		t.Fatalf("failed to generate %s token: %v", role, err)
	}
	return token
}

// do performs a request and returns the recorder.
func do(r *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	var reqBody *bytes.Reader
	if body != "" {
		reqBody = bytes.NewReader([]byte(body))
	} else {
		reqBody = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// errBody extracts the {"error": "..."} message from a response.
func errBody(t *testing.T, body string) string {
	t.Helper()
	var parsed struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal([]byte(body), &parsed)
	return parsed.Error
}

// routeGate describes one protected route and its allowed roles.
type routeGate struct {
	name       string
	method     string
	path       string
	allowed    []string // roles allowed to reach the handler
	disallowed string   // representative role that must get 403
	probePath  string   // exact status expected when an allowed role hits (0 = only assert handler ran)
	probeWant  int
}

func TestRouterAuthz_NoToken401(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, _ := authzTestRouter(t)

	// Every route in scope now requires a bearer token.
	routes := []routeGate{
		{"reservation create", "POST", "/api/reservation/", []string{"client"}, "beauty_center", "", 0},
		{"reservation admin create", "POST", "/api/reservation/admin", []string{"beauty_center", "personnel"}, "client", "", 0},
		{"reservation find all", "GET", "/api/reservation", []string{"beauty_center"}, "client", "", 0},
		{"reservation by id", "GET", "/api/reservation/1", []string{"client", "beauty_center"}, "personnel", "", 0},
		{"reservation by client", "GET", "/api/reservation/client/1", []string{"client"}, "beauty_center", "", 0},
		{"reservation upcoming", "GET", "/api/reservation/client/1/upcoming", []string{"client"}, "beauty_center", "", 0},
		{"reservation by center", "GET", "/api/reservation/center/1", []string{"beauty_center", "personnel"}, "client", "", 0},
		{"reservation update", "PUT", "/api/reservation/1", []string{"client", "beauty_center"}, "personnel", "", 0},
		{"reservation delete", "DELETE", "/api/reservation/1", []string{"client", "beauty_center"}, "personnel", "", 0},
		{"reservation status", "PATCH", "/api/reservation/1/status", []string{"beauty_center"}, "client", "", 0},
		{"reservation assign", "PATCH", "/api/reservation/1/assign", []string{"beauty_center"}, "client", "", 0},
		{"reservation cancel", "PATCH", "/api/reservation/1/cancel", []string{"client"}, "beauty_center", "", 0},
		{"reservation generate qr", "POST", "/api/reservation/generate-qr/1", []string{"client", "beauty_center"}, "personnel", "", 0},
		{"reservation verify qr", "GET", "/api/reservation/verify-qr", []string{"beauty_center", "personnel"}, "client", "", 0},
		{"reservation my agenda", "GET", "/api/reservation/me", []string{"personnel"}, "client", "", 0},
		{"personnel update me", "PATCH", "/api/personnel/me", []string{"personnel"}, "client", "", 0},
		{"client find all", "GET", "/api/client", []string{"beauty_center"}, "client", "", 0},
		{"client by id", "GET", "/api/client/1", []string{"client"}, "beauty_center", "", 0},
		{"personnel find by id", "GET", "/api/personnel/1", []string{"client"}, "client", "", 0},
		{"personnel find all", "GET", "/api/personnel", []string{"client"}, "client", "", 0},
		{"personnel update", "PUT", "/api/personnel/1", []string{"beauty_center"}, "client", "", 0},
		{"personnel delete", "DELETE", "/api/personnel/1", []string{"beauty_center"}, "client", "", 0},
		{"notifications by client", "GET", "/api/notifications/client/1", []string{"client"}, "client", "", 0},
		{"notifications unread", "GET", "/api/notifications/client/1/unread", []string{"client"}, "client", "", 0},
		{"notifications count", "GET", "/api/notifications/client/1/count", []string{"client"}, "client", "", 0},
		{"notification read", "PATCH", "/api/notifications/1/read", []string{"client"}, "client", "", 0},
		{"notifications read all", "POST", "/api/notifications/client/1/read-all", []string{"client"}, "client", "", 0},
		{"push device register", "POST", "/api/push-devices/register", []string{"client"}, "client", "", 0},
		{"notifications center list", "GET", "/api/notifications/center", []string{"beauty_center"}, "client", "", 0},
		{"notifications center unread", "GET", "/api/notifications/center/unread", []string{"beauty_center"}, "client", "", 0},
		{"notifications center count", "GET", "/api/notifications/center/count", []string{"beauty_center"}, "client", "", 0},
		{"notification center read", "PATCH", "/api/notifications/center/1/read", []string{"beauty_center"}, "client", "", 0},
		{"topup process", "PUT", "/api/admin/tokens/topups/1", []string{"beauty_center"}, "client", "", 0},
	}

	for _, rt := range routes {
		t.Run(rt.name+" no token", func(t *testing.T) {
			w := do(r, rt.method, rt.path, "", "")
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			// Handler must not have run: the body is the middleware's, not the controller's.
			assert.Equal(t, "Authorization header missing", errBody(t, w.Body.String()))
		})
	}
}

func TestRouterAuthz_RoleGates(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, _ := authzTestRouter(t)

	routes := []routeGate{
		{"reservation create", "POST", "/api/reservation/", []string{"client"}, "beauty_center", "", 0},
		{"reservation admin create", "POST", "/api/reservation/admin", []string{"beauty_center", "personnel"}, "client", "", 0},
		{"reservation find all", "GET", "/api/reservation", []string{"beauty_center"}, "client", "", 200},
		{"reservation by client", "GET", "/api/reservation/client/1", []string{"client"}, "beauty_center", "", 200},
		{"reservation upcoming", "GET", "/api/reservation/client/1/upcoming", []string{"client"}, "beauty_center", "", 404},
		{"reservation by center", "GET", "/api/reservation/center/1", []string{"beauty_center", "personnel"}, "client", "", 200},
		{"reservation by id", "GET", "/api/reservation/1", []string{"client", "beauty_center"}, "personnel", "", 0},
		{"reservation update", "PUT", "/api/reservation/1", []string{"client", "beauty_center"}, "personnel", "", 0},
		{"reservation delete", "DELETE", "/api/reservation/1", []string{"client", "beauty_center"}, "personnel", "", 0},
		{"reservation status", "PATCH", "/api/reservation/1/status", []string{"beauty_center"}, "client", "", 0},
		{"reservation assign", "PATCH", "/api/reservation/1/assign", []string{"beauty_center"}, "client", "", 0},
		{"reservation cancel", "PATCH", "/api/reservation/1/cancel", []string{"client"}, "beauty_center", "", 0},
		{"reservation generate qr", "POST", "/api/reservation/generate-qr/1", []string{"client", "beauty_center"}, "personnel", "", 0},
		{"reservation verify qr", "GET", "/api/reservation/verify-qr", []string{"beauty_center", "personnel"}, "client", "", 0},
		{"reservation my agenda", "GET", "/api/reservation/me", []string{"personnel"}, "client", "", 200},
		{"personnel update me", "PATCH", "/api/personnel/me", []string{"personnel"}, "client", "", 0},
		{"client find all", "GET", "/api/client", []string{"beauty_center"}, "client", "", 200},
		{"personnel update", "PUT", "/api/personnel/1", []string{"beauty_center"}, "client", "", 0},
		{"personnel delete", "DELETE", "/api/personnel/1", []string{"beauty_center"}, "client", "", 0},
		{"notifications center list", "GET", "/api/notifications/center", []string{"beauty_center"}, "client", "", 200},
		{"notifications center unread", "GET", "/api/notifications/center/unread", []string{"beauty_center"}, "client", "", 200},
		{"notifications center count", "GET", "/api/notifications/center/count", []string{"beauty_center"}, "client", "", 200},
		{"notification center read", "PATCH", "/api/notifications/center/1/read", []string{"beauty_center"}, "client", "", 404},
		{"topup process", "PUT", "/api/admin/tokens/topups/1", []string{"beauty_center"}, "client", "", 0},
	}

	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			disallowedToken := roleToken(t, 1, 0, rt.disallowed)
			w := do(r, rt.method, rt.path, disallowedToken, "")
			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Equal(t, "forbidden", errBody(t, w.Body.String()))

			for _, role := range rt.allowed {
				allowedToken := roleToken(t, 1, 1, role)
				w := do(r, rt.method, rt.path, allowedToken, "")
				assert.NotEqual(t, http.StatusUnauthorized, w.Code)
				assert.NotEqual(t, http.StatusForbidden, w.Code)
				assert.NotEqual(t, "forbidden", errBody(t, w.Body.String()))
				if rt.probeWant != 0 {
					assert.Equal(t, rt.probeWant, w.Code, "allowed role %s should reach handler", role)
				}
			}
		})
	}
}

// ──────────────────────────────────────────────
//  Personnel self-service behavioral coverage
// ──────────────────────────────────────────────

// TestRouterAuthz_PersonnelAgendaScoped — GET /reservation/me returns only the
// caller's own reservations (token claim), never another staff member's, and a
// client token is denied with 403. This is the D1 "no cross-staff surface"
// guarantee at the HTTP layer.
func TestRouterAuthz_PersonnelAgendaScoped(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := authzTestRouter(t)

	today := time.Now().Truncate(24 * time.Hour)
	rows := []models.Reservation{
		{BeautyCenterID: 1, ClientID: 1, PersonnelID: 7, Date: today, StartTime: "09:00", EndTime: "10:00", TotalPrice: 100},
		{BeautyCenterID: 1, ClientID: 2, PersonnelID: 9, Date: today, StartTime: "10:00", EndTime: "11:00", TotalPrice: 200},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("failed to seed: %v", err)
	}

	// Client token → 403 (role gate, not data scoping).
	w := do(r, http.MethodGet, "/api/reservation/me", roleToken(t, 1, 0, "client"), "")
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Personnel 7 → 200, only its own row, no leak of personnel 9's row.
	w = do(r, http.MethodGet, "/api/reservation/me", roleToken(t, 7, 1, "personnel"), "")
	assert.Equal(t, http.StatusOK, w.Code)
	var got []response.ReservationResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 1, "exactly one reservation for personnel 7")
	assert.Equal(t, uint(7), got[0].PersonnelID)
	assert.Equal(t, uint(1), got[0].ClientID)
}

// TestRouterAuthz_PersonnelVerifyQRWiden — verify-qr is reachable by a
// personnel token and returns the VerifyQRResponse envelope (200), while the
// QR it scans must belong to the personnel token's own center (centerID claim).
func TestRouterAuthz_PersonnelVerifyQRWiden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := authzTestRouter(t)

	// The real token-account chain needs an account with balance for the
	// 3.5% charge (TotalPrice 100 → 3.5 YOTTUs → 3500 milli).
	if err := db.Create(&models.TokenAccount{BeautyCenterID: 1, Balance: 100000}).Error; err != nil {
		t.Fatalf("failed to seed token account: %v", err)
	}

	future := time.Now().Add(30 * time.Minute)
	res := &models.Reservation{
		BeautyCenterID:    1,
		ClientID:          1,
		PersonnelID:       1,
		Status:            "CONFIRMED",
		TotalPrice:        100,
		VerificationToken: ptrString("AGENDA-TOK"),
		TokenExpiresAt:    &future,
	}
	if err := db.Create(res).Error; err != nil {
		t.Fatalf("failed to seed: %v", err)
	}

	w := do(r, http.MethodGet, "/api/reservation/verify-qr?token=AGENDA-TOK", roleToken(t, 1, 1, "personnel"), "")
	assert.Equal(t, http.StatusOK, w.Code)
	var vr response.VerifyQRResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &vr))
	assert.Equal(t, res.ID, vr.Reservation.ID)
	assert.False(t, vr.AlreadyCompleted, "first scan completes → already_completed=false")
}

// TestRouterAuthz_PersonnelStatusStillForbidden — widening verify-qr must NOT
// widen /status: a personnel token hitting PATCH /:id/status is still 403
// (design D4 isolation; /status stays beauty_center-only).
func TestRouterAuthz_PersonnelStatusStillForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := authzTestRouter(t)

	res := &models.Reservation{BeautyCenterID: 1, ClientID: 1, PersonnelID: 1, Status: "PENDING", TotalPrice: 50}
	if err := db.Create(res).Error; err != nil {
		t.Fatalf("failed to seed: %v", err)
	}

	w := do(r, http.MethodPatch, "/api/reservation/1/status", roleToken(t, 1, 1, "personnel"), `{"status":"CONFIRMED"}`)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Confirm the behavior is unchanged for the legitimate beauty_center role.
	w = do(r, http.MethodPatch, "/api/reservation/1/status", roleToken(t, 1, 1, "beauty_center"), `{"status":"CONFIRMED"}`)
	assert.Equal(t, http.StatusOK, w.Code)
}
