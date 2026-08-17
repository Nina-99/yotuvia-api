package tests

import (
	"encoding/json"
	"net/http"
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

// Fixed date literals mirror the spec scenarios (design D3: dates are compared
// as date-only values, so seeds use compatible UTC midnights).
var (
	centerDateDayOne = time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	centerDateDayTwo = time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	centerDateEmpty  = time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC)
)

// centerDateSeed groups the reservations relevant to the date-scoping and
// ownership assertions so the tests reference real created IDs instead of
// hardcoding row numbers.
type centerDateSeed struct {
	dayOne      []*models.Reservation // center 1, 2026-08-16: 09:00 then 14:00
	dayTwo      *models.Reservation   // center 1, 2026-08-17: 10:00
	otherCenter *models.Reservation   // center 2, 2026-08-16: 11:00 — must never leak
}

// seedCenterDateData seeds two centers with reservations across two dates:
//
//	center 1: 08-16 @ 09:00, 08-16 @ 14:00, 08-17 @ 10:00
//	center 2: 08-16 @ 11:00              ← cross-center leak guard
//
// Services/personnel are seeded through the shared dashboard helpers so the
// preloads resolve like production.
func seedCenterDateData(t *testing.T, db *gorm.DB) centerDateSeed {
	t.Helper()
	seedDashboardCenter(t, db, 1, "Salon One", "one@test.com", true)
	seedDashboardCenter(t, db, 2, "Salon Two", "two@test.com", true)
	for i := 1; i <= 3; i++ {
		seedDashboardClient(t, db, uint(i), "CenterClient", string(rune('0'+i)))
	}
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 2, true)
	seedDashboardService(t, db, 1, 1, "Corte", true)
	seedDashboardService(t, db, 2, 2, "Color", true)

	var seed centerDateSeed
	seed.dayOne = append(seed.dayOne,
		seedReservationWithServices(t, db, 1, seedReservationPlan{
			date: centerDateDayOne, startTime: "09:00", endTime: "10:00",
			status: "PENDING", totalPrice: 100, clientID: 1, personnelID: 1, serviceIDs: []uint{1},
		}),
		seedReservationWithServices(t, db, 1, seedReservationPlan{
			date: centerDateDayOne, startTime: "14:00", endTime: "15:00",
			status: "CONFIRMED", totalPrice: 80, clientID: 2, personnelID: 1, serviceIDs: []uint{1},
		}),
	)
	seed.dayTwo = seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: centerDateDayTwo, startTime: "10:00", endTime: "11:00",
		status: "PENDING", totalPrice: 120, clientID: 3, personnelID: 1, serviceIDs: []uint{1},
	})
	seed.otherCenter = seedReservationWithServices(t, db, 2, seedReservationPlan{
		date: centerDateDayOne, startTime: "11:00", endTime: "12:00",
		status: "PENDING", totalPrice: 60, clientID: 1, personnelID: 2, serviceIDs: []uint{2},
	})
	return seed
}

// reservationCenterTestRouter mounts ReservationRoutes exactly as authzTestRouter
// does, mirroring the focused-router pattern of dashboardTestRouter.
func reservationCenterTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	validate := validator.New()
	r := gin.New()
	rg := r.Group("/api")
	router.ReservationRoutes(rg, db, validate)
	return r, db
}

// centerReservationLite is the subset of the ReservationResponse DTO the date
// scenarios assert on.
type centerReservationLite struct {
	ID             uint      `json:"id"`
	BeautyCenterID uint      `json:"beauty_center_id"`
	Date           time.Time `json:"date"`
	StartTime      string    `json:"start_time"`
}

// TestReservationCenter_DateScoping — GET /reservation/center/:centerId with
// the optional date query parameter (spec: Date-Scoped Center Reservations
// Querying). Covers all five spec scenarios: no date (all returned, unfiltered
// and unordered), valid date (only that day, start_time ASC), malformed date
// (400), empty day (200 []), and no cross-center leakage.
func TestReservationCenter_DateScoping(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reservationCenterTestRouter(t)
	seed := seedCenterDateData(t, db)

	centerOneToken := roleToken(t, 7, 1, "beauty_center")

	tests := []struct {
		name           string
		queryPath      string
		wantStatus     int
		wantLen        int
		wantAllCenterOne bool   // no-date scenario: assert the full set, order agnostic
		wantIDs        []uint // ordered exact match (valid-date scenario)
		wantStartTimes []string
		wantErr        string // non-empty asserts the {"error": ...} message
	}{
		{
			name:            "no date returns all center reservations unfiltered",
			queryPath:       "/api/reservation/center/1",
			wantStatus:      http.StatusOK,
			wantLen:         3,
			wantAllCenterOne: true,
		},
		{
			name:           "valid date filters to that day ordered by start_time ASC",
			queryPath:      "/api/reservation/center/1?date=2026-08-16",
			wantStatus:     http.StatusOK,
			wantLen:        2,
			wantIDs:        []uint{seed.dayOne[0].ID, seed.dayOne[1].ID},
			wantStartTimes: []string{"09:00", "14:00"},
		},
		{
			name:       "malformed date returns 400",
			queryPath:  "/api/reservation/center/1?date=invalid-date",
			wantStatus: http.StatusBadRequest,
			wantErr:    "bad request",
		},
		{
			name:       "date with no reservations returns empty array",
			queryPath:  "/api/reservation/center/1?date=2026-12-25",
			wantStatus: http.StatusOK,
			wantLen:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := do(r, http.MethodGet, tt.queryPath, centerOneToken, "")
			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantErr != "" {
				assert.Equal(t, tt.wantErr, errBody(t, w.Body.String()))
				return
			}

			var list []centerReservationLite
			assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
			if !assert.Len(t, list, tt.wantLen) {
				return // length mismatch — nothing further to assert
			}
			if tt.wantLen == 0 {
				// Wire contract: an empty result must serialize as [] — never null.
				assert.Contains(t, w.Body.String(), `[]`)
				return
			}

			if tt.wantAllCenterOne {
				// No-date behavior preserves the current unordered response:
				// assert set membership, never order, and never center 2's row.
				var gotIDs []uint
				for _, r := range list {
					gotIDs = append(gotIDs, r.ID)
					assert.Equal(t, uint(1), r.BeautyCenterID, "no cross-center leakage")
				}
				assert.ElementsMatch(t,
					[]uint{seed.dayOne[0].ID, seed.dayOne[1].ID, seed.dayTwo.ID},
					gotIDs, "all center-1 reservations across both dates")
				return
			}

			// Valid-date scenario: exact day membership + start_time ASC order.
			for i, r := range list {
				assert.Equal(t, tt.wantIDs[i], r.ID, "order must follow start_time ASC")
				assert.Equal(t, tt.wantStartTimes[i], r.StartTime)
				assert.True(t, r.Date.Equal(centerDateDayOne), "every row belongs to the queried day")
			}
		})
	}
}

// TestReservationCenter_DateAuthz — authorization and ownership enforcement is
// unchanged (spec: a personnel or beauty_center token of center ID 1 querying
// center 2 is 403). The client role is still blocked by the route gate.
func TestReservationCenter_DateAuthz(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reservationCenterTestRouter(t)
	seedCenterDateData(t, db)

	pathDayOne := "/api/reservation/center/1?date=2026-08-16"
	pathOtherCenter := "/api/reservation/center/2?date=2026-08-16"

	tests := []struct {
		name       string
		token      string
		path       string
		wantStatus int
		wantErr    string
	}{
		{
			name:       "client role is 403 (route gate)",
			token:      roleToken(t, 1, 0, "client"),
			path:       pathDayOne,
			wantStatus: http.StatusForbidden,
			wantErr:    "forbidden",
		},
		{
			name:       "beauty_center cross-center ownership is 403",
			token:      roleToken(t, 7, 1, "beauty_center"),
			path:       pathOtherCenter,
			wantStatus: http.StatusForbidden,
			wantErr:    "forbidden",
		},
		{
			name:       "personnel cross-center ownership is 403",
			token:      roleToken(t, 1, 1, "personnel"),
			path:       pathOtherCenter,
			wantStatus: http.StatusForbidden,
			wantErr:    "forbidden",
		},
		{
			name:       "personnel missing centerID claim is 403",
			token:      roleToken(t, 1, 0, "personnel"),
			path:       pathDayOne,
			wantStatus: http.StatusForbidden,
			wantErr:    "forbidden",
		},
		{
			name:       "personnel own center with date is 200",
			token:      roleToken(t, 1, 1, "personnel"),
			path:       pathDayOne,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := do(r, http.MethodGet, tt.path, tt.token, "")
			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantErr != "" {
				assert.Equal(t, tt.wantErr, errBody(t, w.Body.String()))
			}
		})
	}
}