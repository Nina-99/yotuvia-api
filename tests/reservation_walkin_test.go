package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/router"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Fixed date literal mirrors the spec scenarios (design D3: dates are compared
// as date-only values, so seeds use compatible UTC midnights).
var walkinDay = time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)

// walkinSeed groups the walk-in fixtures so the tests reference real created
// rows. Services are seeded with the dashboard helper's 30min/$50 profile:
//
//	center 1: service 1 (Corte) + service 3 (Peinado) → 60min/$100 combined
//	center 2: service 2 (Color) — cross-center rejection guard
type walkinSeed struct {
	centerOne   *models.BeautyCenter
	centerTwo   *models.BeautyCenter
	staffOne    *models.Personnel   // belongs to center 1
	staffTwo    *models.Personnel   // belongs to center 2
	svcOne      *models.Service     // center 1, 30min, $50
	svcTwo      *models.Service     // center 2, 30min, $50 — must never book at center 1
	svcThree    *models.Service     // center 1, 30min, $50
	overlapSlot *models.Reservation // center 1, 08-16 09:00–10:00, staff 1
}

// seedWalkinData seeds two centers, five active clients, one inactive client,
// personnel for each center, and the service catalog through the shared
// dashboard helpers so the preloads resolve like production.
func seedWalkinData(t *testing.T, db *gorm.DB) walkinSeed {
	t.Helper()
	var seed walkinSeed
	seed.centerOne = seedDashboardCenter(t, db, 1, "Salon One", "one@test.com", true)
	seed.centerTwo = seedDashboardCenter(t, db, 2, "Salon Two", "two@test.com", true)
	for i := 1; i <= 5; i++ {
		seedDashboardClient(t, db, uint(i), "Walkin", string(rune('0'+i)))
	}
	inactive := seedDashboardClient(t, db, 6, "Walkin", "6")
	if err := db.Model(inactive).Update("is_active", false).Error; err != nil {
		t.Fatalf("failed to force client inactive: %v", err)
	}

	seed.staffOne = seedDashboardPersonnel(t, db, 1, 1, true)
	seed.staffTwo = seedDashboardPersonnel(t, db, 2, 2, true)
	seed.svcOne = seedDashboardService(t, db, 1, 1, "Corte", true)
	seed.svcTwo = seedDashboardService(t, db, 2, 2, "Color", true)
	seed.svcThree = seedDashboardService(t, db, 3, 1, "Peinado", true)

	// The client self-booking path enforces the center's posted schedule, so
	// publish Sunday hours covering the windows the client scenarios book
	// (walkinDay is a Sunday). Admin/walk-in scenarios ignore these hours.
	seedWorkingHour(t, db, 1, int(walkinDay.Weekday()), "08:00", "20:00")

	seed.overlapSlot = seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: walkinDay, startTime: "09:00", endTime: "10:00",
		status: "PENDING", totalPrice: 50, clientID: 2, personnelID: 1, serviceIDs: []uint{1},
	})
	return seed
}

// reservationWalkinTestRouter mounts ReservationRoutes exactly as
// reservationCenterTestRouter does, mirroring the focused-router pattern.
func reservationWalkinTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	validate := validator.New()
	r := gin.New()
	rg := r.Group("/api")
	router.ReservationRoutes(rg, db, validate)
	return r, db
}

// walkinReservationLite is the subset of the ReservationResponse DTO the
// walk-in scenarios assert on (design D6: 201 + computed DTO).
type walkinReservationLite struct {
	ID               uint    `json:"id"`
	BeautyCenterID   uint    `json:"beauty_center_id"`
	BeautyCenterName string  `json:"beauty_center_name"`
	ClientID         uint    `json:"client_id"`
	ClientName       string  `json:"client_name"`
	PersonnelID      uint    `json:"personnel_id"`
	StartTime        string  `json:"start_time"`
	EndTime          string  `json:"end_time"`
	TotalPrice       float64 `json:"total_price"`
	Status           string  `json:"status"`
	Services         []struct {
		ServiceID   uint    `json:"service_id"`
		ServiceName string  `json:"service_name"`
		Price       float64 `json:"price"`
		Duration    int     `json:"duration"`
	} `json:"services"`
}

func postWalkin(r *gin.Engine, token, body string) *httptest.ResponseRecorder {
	return do(r, http.MethodPost, "/api/reservation/admin", token, body)
}

// TestReservationWalkin_CreateSuccess — POST /reservation/admin with a
// beauty_center token creates a walk-in for a provided active client and
// returns 201 with the computed ReservationResponse DTO (spec: Successful
// admin walk-in reservation; design D6). The table forces real computation:
// different service sets yield different end_time/total_price, and the DB rows
// are checked for the same persisted values.
func TestReservationWalkin_CreateSuccess(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reservationWalkinTestRouter(t)
	seedWalkinData(t, db)

	adminToken := roleToken(t, 7, 1, "beauty_center")

	tests := []struct {
		name        string
		body        string
		wantClient  uint
		wantEnd     string
		wantTotal   float64
		wantServLen int
		wantSvcIDs  []uint
	}{
		{
			name:        "two services compute end_time and total_price",
			body:        `{"beauty_center_id":1,"client_id":1,"personnel_id":1,"service_ids":[1,3],"date":"2026-08-16","start_time":"10:30"}`,
			wantClient:  1,
			wantEnd:     "11:30",
			wantTotal:   100,
			wantServLen: 2,
			wantSvcIDs:  []uint{1, 3},
		},
		{
			name:        "single service keeps price and duration per service",
			body:        `{"beauty_center_id":1,"client_id":3,"personnel_id":1,"service_ids":[1],"date":"2026-08-16","start_time":"13:00"}`,
			wantClient:  3,
			wantEnd:     "13:30",
			wantTotal:   50,
			wantServLen: 1,
			wantSvcIDs:  []uint{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := postWalkin(r, adminToken, tt.body)
			if !assert.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String()) {
				return
			}

			var resp walkinReservationLite
			if !assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp)) {
				return
			}
			assert.NotZero(t, resp.ID, "created reservation carries a real id")
			assert.Equal(t, uint(1), resp.BeautyCenterID)
			assert.Equal(t, "Salon One", resp.BeautyCenterName)
			assert.Equal(t, tt.wantClient, resp.ClientID, "client comes from the body, never the admin claim")
			assert.Equal(t, "Walkin "+string(rune('0'+tt.wantClient)), resp.ClientName)
			assert.Equal(t, uint(1), resp.PersonnelID)
			assert.Equal(t, tt.wantEnd, resp.EndTime, "end_time = start + sum of service durations")
			assert.Equal(t, tt.wantTotal, resp.TotalPrice, "total_price = sum of service prices")
			assert.Equal(t, "PENDING", resp.Status, "walk-ins start PENDING")
			if assert.Len(t, resp.Services, tt.wantServLen) {
				var gotIDs []uint
				for _, s := range resp.Services {
					gotIDs = append(gotIDs, s.ServiceID)
					assert.Equal(t, float64(50), s.Price)
					assert.Equal(t, 30, s.Duration)
				}
				assert.ElementsMatch(t, tt.wantSvcIDs, gotIDs)
			}

			// The same computed values must be persisted, not just echoed.
			var stored models.Reservation
			if err := db.First(&stored, resp.ID).Error; err != nil {
				t.Fatalf("failed to reload reservation: %v", err)
			}
			assert.True(t, stored.Date.Equal(walkinDay))
			assert.Equal(t, tt.wantEnd, stored.EndTime)
			assert.Equal(t, tt.wantTotal, stored.TotalPrice)
			assert.Equal(t, "PENDING", stored.Status)
			assert.Equal(t, tt.wantClient, stored.ClientID)

			var rs []models.ReservationService
			if err := db.Where("reservation_id = ?", resp.ID).Find(&rs).Error; err != nil {
				t.Fatalf("failed to reload reservation services: %v", err)
			}
			if assert.Len(t, rs, tt.wantServLen) {
				for _, row := range rs {
					assert.Equal(t, float64(50), row.Price)
					assert.Equal(t, 30, row.Duration)
				}
			}
		})
	}
}

// TestReservationWalkin_Authz — the actor-bug regression (spec: Client
// creation by non-client role): POST /reservation/ is client-only, so
// beauty_center and personnel tokens are 403; a client token still creates
// (existing behavior, 201). POST /reservation/admin is the inverse: a client
// token is 403.
func TestReservationWalkin_Authz(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reservationWalkinTestRouter(t)
	seedWalkinData(t, db)

	createBody := `{"beauty_center_id":1,"personnel_id":1,"service_ids":[1],"date":"2026-08-16","start_time":"12:00"}`

	tests := []struct {
		name       string
		method     string
		path       string
		token      string
		body       string
		wantStatus int
		wantErr    string
	}{
		{
			name:       "beauty_center cannot create via client endpoint",
			method:     http.MethodPost,
			path:       "/api/reservation/",
			token:      roleToken(t, 7, 1, "beauty_center"),
			body:       createBody,
			wantStatus: http.StatusForbidden,
			wantErr:    "forbidden",
		},
		{
			name:       "personnel cannot create via client endpoint",
			method:     http.MethodPost,
			path:       "/api/reservation/",
			token:      roleToken(t, 1, 1, "personnel"),
			body:       createBody,
			wantStatus: http.StatusForbidden,
			wantErr:    "forbidden",
		},
		{
			name:       "client still creates via client endpoint",
			method:     http.MethodPost,
			path:       "/api/reservation/",
			token:      roleToken(t, 1, 0, "client"),
			body:       createBody,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "client cannot create via admin endpoint",
			method:     http.MethodPost,
			path:       "/api/reservation/admin",
			token:      roleToken(t, 1, 0, "client"),
			body:       createBody,
			wantStatus: http.StatusForbidden,
			wantErr:    "forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := do(r, tt.method, tt.path, tt.token, tt.body)
			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantErr != "" {
				assert.Equal(t, tt.wantErr, errBody(t, w.Body.String()))
			}
		})
	}
}

// TestReservationWalkin_AdminValidations — POST /reservation/admin rejects
// missing/nonexistent/inactive clients, services not belonging to the center,
// empty service lists, and malformed date/time (spec: Nonexistent client
// validation + 400 contract). All use a beauty_center token of center 1.
func TestReservationWalkin_AdminValidations(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reservationWalkinTestRouter(t)
	seedWalkinData(t, db)

	adminToken := roleToken(t, 7, 1, "beauty_center")

	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "missing client_id is 400",
			body:    `{"beauty_center_id":1,"personnel_id":1,"service_ids":[1],"date":"2026-08-16","start_time":"09:30"}`,
			wantErr: "validation failed",
		},
		{
			name:    "nonexistent client is 400",
			body:    `{"beauty_center_id":1,"client_id":999,"personnel_id":1,"service_ids":[1],"date":"2026-08-16","start_time":"09:30"}`,
			wantErr: "bad request",
		},
		{
			name:    "inactive client is 400",
			body:    `{"beauty_center_id":1,"client_id":6,"personnel_id":1,"service_ids":[1],"date":"2026-08-16","start_time":"09:30"}`,
			wantErr: "bad request",
		},
		{
			name:    "empty service_ids is 400",
			body:    `{"beauty_center_id":1,"client_id":1,"personnel_id":1,"service_ids":[],"date":"2026-08-16","start_time":"09:30"}`,
			wantErr: "validation failed",
		},
		{
			name:    "service of another center is 400",
			body:    `{"beauty_center_id":1,"client_id":1,"personnel_id":1,"service_ids":[2],"date":"2026-08-16","start_time":"09:30"}`,
			wantErr: "bad request",
		},
		{
			name:    "malformed date is 400",
			body:    `{"beauty_center_id":1,"client_id":1,"personnel_id":1,"service_ids":[1],"date":"16/08/2026","start_time":"09:30"}`,
			wantErr: "bad request",
		},
		{
			name:    "malformed start_time is 400",
			body:    `{"beauty_center_id":1,"client_id":1,"personnel_id":1,"service_ids":[1],"date":"2026-08-16","start_time":"25:99"}`,
			wantErr: "bad request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := postWalkin(r, adminToken, tt.body)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Equal(t, tt.wantErr, errBody(t, w.Body.String()))
		})
	}
}

// TestReservationWalkin_CenterBinding — D2 actor binding: the JWT centerID
// claim is the source of truth (body beauty_center_id must match), personnel
// membership is proven via the Personnel row, and a missing center claim can
// never book (spec: Admin creation with mismatched center ID / Personnel
// creation for unauthorized center).
func TestReservationWalkin_CenterBinding(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reservationWalkinTestRouter(t)
	seedWalkinData(t, db)

	validBody := `{"beauty_center_id":1,"client_id":4,"personnel_id":1,"service_ids":[1],"date":"2026-08-16","start_time":"13:00"}`

	tests := []struct {
		name    string
		token   string
		body    string
		want    int
		wantErr string
	}{
		{
			name:  "beauty_center books own center",
			token: roleToken(t, 7, 1, "beauty_center"),
			body:  validBody,
			want:  http.StatusCreated,
		},
		{
			name:    "beauty_center body center mismatches claim is 403",
			token:   roleToken(t, 7, 1, "beauty_center"),
			body:    `{"beauty_center_id":2,"client_id":1,"personnel_id":1,"service_ids":[2],"date":"2026-08-16","start_time":"09:30"}`,
			want:    http.StatusForbidden,
			wantErr: "forbidden",
		},
		{
			name:    "beauty_center without center claim is 403",
			token:   roleToken(t, 7, 0, "beauty_center"),
			body:    validBody,
			want:    http.StatusForbidden,
			wantErr: "forbidden",
		},
		{
			name:  "personnel of own center books",
			token: roleToken(t, 1, 1, "personnel"),
			body:  `{"beauty_center_id":1,"client_id":5,"personnel_id":1,"service_ids":[1],"date":"2026-08-16","start_time":"14:00"}`,
			want:  http.StatusCreated,
		},
		{
			name:    "personnel body center mismatches claim is 403",
			token:   roleToken(t, 1, 1, "personnel"),
			body:    `{"beauty_center_id":2,"client_id":1,"personnel_id":1,"service_ids":[2],"date":"2026-08-16","start_time":"09:30"}`,
			want:    http.StatusForbidden,
			wantErr: "forbidden",
		},
		{
			name:    "personnel row not member of claimed center is 403",
			token:   roleToken(t, 2, 1, "personnel"),
			body:    validBody,
			want:    http.StatusForbidden,
			wantErr: "forbidden",
		},
		{
			name:    "personnel without center claim is 403",
			token:   roleToken(t, 1, 0, "personnel"),
			body:    validBody,
			want:    http.StatusForbidden,
			wantErr: "forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := postWalkin(r, tt.token, tt.body)
			assert.Equal(t, tt.want, w.Code)
			if tt.wantErr != "" {
				assert.Equal(t, tt.wantErr, errBody(t, w.Body.String()))
			}
		})
	}
}

// TestReservationWalkin_SlotConflict — a walk-in overlapping an existing
// reservation at the same center/day returns 409 (spec: Time slot overlap
// conflict). The seeded 09:00–10:00 slot collides with a 09:00–09:30 walk-in.
func TestReservationWalkin_SlotConflict(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reservationWalkinTestRouter(t)
	// seedWalkinData plants overlapSlot (08-16 09:00–10:00, staff 1).
	seedWalkinData(t, db)

	adminToken := roleToken(t, 7, 1, "beauty_center")
	body := `{"beauty_center_id":1,"client_id":1,"personnel_id":1,"service_ids":[1],"date":"2026-08-16","start_time":"09:00"}`

	w := postWalkin(r, adminToken, body)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "conflict", errBody(t, w.Body.String()))

	// Exactly one reservation remains in the slot: the seeded one.
	var count int64
	if err := db.Model(&models.Reservation{}).Where("beauty_center_id = ? AND date = ? AND start_time = ?", 1, walkinDay, "09:00").Count(&count).Error; err != nil {
		t.Fatalf("failed to count reservations: %v", err)
	}
	assert.Equal(t, int64(1), count, "conflicting walk-in must not be persisted")
}
