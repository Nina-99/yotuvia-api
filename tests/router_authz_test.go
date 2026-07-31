package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/router"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupRouterAuthzDB migrates the same model set configs/db.config.go does, so
// allowed-role probes can actually reach their handlers on an empty database.
func setupRouterAuthzDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	modelsToMigrate := []interface{}{
		&models.AuthAccount{}, &models.BeautyCenter{}, &models.BeautyCenterImage{},
		&models.Category{}, &models.TokenAccount{}, &models.TokenTopUp{},
		&models.TokenTransaction{}, &models.TokenMission{}, &models.TokenMissionProgress{},
		&models.Referral{}, &models.Client{}, &models.CustomerPunchCard{},
		&models.Favorite{}, &models.FeedPost{}, &models.LoyaltyProgram{},
		&models.Personnel{}, &models.PersonnelImage{}, &models.PersonnelService{},
		&models.PostLike{}, &models.PunchHistory{}, &models.Reservation{},
		&models.ReservationService{}, &models.Review{}, &models.Service{},
		&models.SocialMedia{}, &models.WorkingHour{}, &models.Video{},
		&models.PersonnelInvitation{}, &models.Notification{}, &models.PushDevice{},
	}
	for _, m := range modelsToMigrate {
		if err := db.AutoMigrate(m); err != nil {
			t.Fatalf("failed to migrate %T: %v", m, err)
		}
	}
	return db
}

// authzTestRouter mounts the four routers exactly as router.go does.
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
		{"reservation verify qr", "GET", "/api/reservation/verify-qr", []string{"beauty_center"}, "client", "", 0},
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
		{"reservation verify qr", "GET", "/api/reservation/verify-qr", []string{"beauty_center"}, "client", "", 0},
		{"client find all", "GET", "/api/client", []string{"beauty_center"}, "client", "", 200},
		{"personnel update", "PUT", "/api/personnel/1", []string{"beauty_center"}, "client", "", 0},
		{"personnel delete", "DELETE", "/api/personnel/1", []string{"beauty_center"}, "client", "", 0},
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
