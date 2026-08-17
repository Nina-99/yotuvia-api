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
	adminClientsDayA = time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	adminClientsDayB = time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	adminClientsDayC = time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
)

// adminClientsSeed groups the reservations relevant to the center-scoped
// client list and the client-scoped detail history so the tests reference real
// created IDs instead of hardcoding row numbers.
//
//	center 1: client 1 (COMPLETED x3), client 2 (CANCELLED), client 3 (PENDING)
//	center 2: client 1 (PENDING — shared client), client 4 (COMPLETED — exclusive)
//	client 5: no reservations anywhere
type adminClientsSeed struct {
	c1DayA     *models.Reservation // center 1, client 1, COMPLETED, 08-10 09:00, 100.00
	c1DayB14   *models.Reservation // center 1, client 1, COMPLETED, 08-15 14:00, 50.00
	c1DayB09   *models.Reservation // center 1, client 1, COMPLETED, 08-15 09:00, 30.00
	c1Center2  *models.Reservation // center 2, client 1, PENDING,   08-15 09:00, 200.00 — shared client leak guard
	c2Cancelled *models.Reservation // center 1, client 2, CANCELLED, 08-10 10:00, 80.00
	c3Pending  *models.Reservation // center 1, client 3, PENDING,   08-20 11:00, 60.00
	c4Center2  *models.Reservation // center 2, client 4, COMPLETED, 08-10 12:00, 90.00 — exclusive client leak guard
}

// seedAdminClientsData seeds two centers, four active clients (one shared, one
// exclusive per center), services, and personnel through the shared dashboard
// helpers so the preloads resolve like production.
func seedAdminClientsData(t *testing.T, db *gorm.DB) adminClientsSeed {
	t.Helper()
	seedDashboardCenter(t, db, 1, "Salon One", "one@test.com", true)
	seedDashboardCenter(t, db, 2, "Salon Two", "two@test.com", true)
	for i := 1; i <= 5; i++ {
		seedDashboardClient(t, db, uint(i), "Client", string(rune('0'+i)))
	}
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 2, true)
	seedDashboardService(t, db, 1, 1, "Corte", true)
	seedDashboardService(t, db, 2, 2, "Color", true)

	var seed adminClientsSeed
	seed.c1DayA = seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: adminClientsDayA, startTime: "09:00", endTime: "10:00",
		status: "COMPLETED", totalPrice: 100, clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})
	seed.c1DayB14 = seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: adminClientsDayB, startTime: "14:00", endTime: "15:00",
		status: "COMPLETED", totalPrice: 50, clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})
	seed.c1DayB09 = seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: adminClientsDayB, startTime: "09:00", endTime: "10:00",
		status: "COMPLETED", totalPrice: 30, clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})
	seed.c1Center2 = seedReservationWithServices(t, db, 2, seedReservationPlan{
		date: adminClientsDayB, startTime: "09:00", endTime: "10:00",
		status: "PENDING", totalPrice: 200, clientID: 1, personnelID: 2, serviceIDs: []uint{2},
	})
	seed.c2Cancelled = seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: adminClientsDayA, startTime: "10:00", endTime: "11:00",
		status: "CANCELLED", totalPrice: 80, clientID: 2, personnelID: 1, serviceIDs: []uint{1},
	})
	seed.c3Pending = seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: adminClientsDayC, startTime: "11:00", endTime: "12:00",
		status: "PENDING", totalPrice: 60, clientID: 3, personnelID: 1, serviceIDs: []uint{1},
	})
	seed.c4Center2 = seedReservationWithServices(t, db, 2, seedReservationPlan{
		date: adminClientsDayA, startTime: "12:00", endTime: "13:00",
		status: "COMPLETED", totalPrice: 90, clientID: 4, personnelID: 2, serviceIDs: []uint{2},
	})
	return seed
}

// adminClientsTestRouter mounts ClientRoutes and ReservationRoutes exactly as
// authzTestRouter does, mirroring the focused-router pattern of
// reservationCenterTestRouter.
func adminClientsTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	validate := validator.New()
	r := gin.New()
	rg := r.Group("/api")
	router.ClientRoutes(rg, db, validate)
	router.ReservationRoutes(rg, db, validate)
	return r, db
}

// adminClientLite is the subset of the AdminClientResponse DTO the list
// scenarios assert on.
type adminClientLite struct {
	ID          uint       `json:"id"`
	FirstName   string     `json:"first_name"`
	LastName    string     `json:"last_name"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	AvatarURL   string     `json:"avatar_url"`
	IsActive    bool       `json:"is_active"`
	VisitsCount int        `json:"visits_count"`
	TotalSpent  float64    `json:"total_spent"`
	LastVisit   *time.Time `json:"last_visit"`
}

// adminClientPage is the PaginatedResponse envelope for the client list.
type adminClientPage struct {
	Data  []adminClientLite `json:"data"`
	Total int64             `json:"total"`
	Page  int               `json:"page"`
	Limit int               `json:"limit"`
}

// adminReservationLite is the subset of the ReservationResponse DTO the
// client-scoped history scenarios assert on.
type adminReservationLite struct {
	ID             uint      `json:"id"`
	BeautyCenterID uint      `json:"beauty_center_id"`
	ClientID       uint      `json:"client_id"`
	Date           time.Time `json:"date"`
	StartTime      string    `json:"start_time"`
}

// TestAdminClients_ScopedListAndMetrics — GET /client with a beauty_center
// token (spec: Client Endpoint Ownership). Covers list membership (only clients
// with ≥1 reservation at the caller's center, any status), the cross-center
// isolation regression (the global-list leak fix proof), and COMPLETED-only
// aggregate metrics (visits_count, total_spent, last_visit).
func TestAdminClients_ScopedListAndMetrics(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := adminClientsTestRouter(t)
	seed := seedAdminClientsData(t, db)

	centerOneToken := roleToken(t, 7, 1, "beauty_center")
	centerTwoToken := roleToken(t, 8, 2, "beauty_center")

	// ── Center 1: only clients 1, 2, 3 (≥1 reservation at center 1) ──
	w := do(r, http.MethodGet, "/api/client", centerOneToken, "")
	assert.Equal(t, http.StatusOK, w.Code)
	var page adminClientPage
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))

	assert.Equal(t, int64(3), page.Total, "total must be the center-scoped count, never the global one")
	assert.Len(t, page.Data, 3)

	byID := make(map[uint]adminClientLite, len(page.Data))
	for _, c := range page.Data {
		byID[c.ID] = c
	}

	// Client 4 (reservations ONLY at center 2) must never leak into center 1.
	assert.NotContains(t, byID, seed.c4Center2.ClientID, "cross-center isolation: center 2's exclusive client leaked")
	// Client 5 (no reservations) must not appear anywhere.
	assert.NotContains(t, byID, uint(5), "client without reservations must not be listed")

	// Client 1: 3 COMPLETED visits → 180.00 spent, last visit 08-15.
	c1, ok := byID[seed.c1DayA.ClientID]
	assert.True(t, ok, "client 1 must be listed (has COMPLETED reservations at center 1)")
	assert.Equal(t, 3, c1.VisitsCount, "visits_count counts only COMPLETED")
	assert.Equal(t, 180.0, c1.TotalSpent, "total_spent sums only COMPLETED")
	if assert.NotNil(t, c1.LastVisit, "last_visit is the newest COMPLETED date") {
		assert.True(t, c1.LastVisit.Equal(adminClientsDayB), "last_visit must be 2026-08-15")
	}

	// Client 2: only CANCELLED at center 1 → still listed, zero metrics.
	c2, ok := byID[seed.c2Cancelled.ClientID]
	assert.True(t, ok, "client with only a CANCELLED reservation must still be listed (any status)")
	assert.Equal(t, 0, c2.VisitsCount, "CANCELLED-only client has zero visits")
	assert.Equal(t, 0.0, c2.TotalSpent, "CANCELLED-only client spent nothing")
	assert.Nil(t, c2.LastVisit, "CANCELLED-only client has no last visit")

	// Client 3: only PENDING at center 1 → listed, zero metrics.
	c3, ok := byID[seed.c3Pending.ClientID]
	assert.True(t, ok, "client with only a PENDING reservation must still be listed (any status)")
	assert.Equal(t, 0, c3.VisitsCount)
	assert.Equal(t, 0.0, c3.TotalSpent)
	assert.Nil(t, c3.LastVisit)

	// ── Center 2: only clients 1 (shared) and 4 (exclusive) ──
	w = do(r, http.MethodGet, "/api/client", centerTwoToken, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))

	assert.Equal(t, int64(2), page.Total)
	byID = make(map[uint]adminClientLite, len(page.Data))
	for _, c := range page.Data {
		byID[c.ID] = c
	}
	assert.Contains(t, byID, seed.c4Center2.ClientID, "client 4 belongs to center 2")
	assert.Contains(t, byID, seed.c1DayA.ClientID, "shared client 1 is listed for center 2 too")
	assert.NotContains(t, byID, seed.c3Pending.ClientID, "center 1's exclusive clients must not leak into center 2")

	c4 := byID[seed.c4Center2.ClientID]
	assert.Equal(t, 1, c4.VisitsCount)
	assert.Equal(t, 90.0, c4.TotalSpent)
	if assert.NotNil(t, c4.LastVisit) {
		assert.True(t, c4.LastVisit.Equal(adminClientsDayA))
	}
}

// TestAdminClients_Pagination — the list envelope honors page/limit defaults
// (1/20), applies limit, and keeps the total scoped to the center across pages.
func TestAdminClients_Pagination(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := adminClientsTestRouter(t)
	seedAdminClientsData(t, db)

	centerOneToken := roleToken(t, 7, 1, "beauty_center")

	// Defaults: page=1, limit=20 → all 3 scoped clients, envelope echoes defaults.
	w := do(r, http.MethodGet, "/api/client", centerOneToken, "")
	assert.Equal(t, http.StatusOK, w.Code)
	var page adminClientPage
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, 1, page.Page, "default page is 1")
	assert.Equal(t, 20, page.Limit, "default limit is 20")
	assert.Len(t, page.Data, 3)

	// limit=2 page=1 → clients 1, 2 (id ASC); total stays the scoped 3.
	w = do(r, http.MethodGet, "/api/client?page=1&limit=2", centerOneToken, "")
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Len(t, page.Data, 2)
	assert.Equal(t, uint(1), page.Data[0].ID)
	assert.Equal(t, uint(2), page.Data[1].ID)
	assert.Equal(t, int64(3), page.Total, "total is the scoped count, not the page size")

	// page=2 limit=2 → the remaining client; still the scoped total.
	w = do(r, http.MethodGet, "/api/client?page=2&limit=2", centerOneToken, "")
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Len(t, page.Data, 1)
	assert.Equal(t, uint(3), page.Data[0].ID)
	assert.Equal(t, int64(3), page.Total)
}

// TestAdminClients_DetailHistory — GET /reservation/center/:centerId with the
// optional clientId query parameter (spec: Client Reservations Scope by Center).
// Covers the client-scoped list, the reverse-chronological order, cross-center
// isolation, malformed/zero clientId (400), the empty case (200 []), and the
// no-clientId regression (date scoping still works).
func TestAdminClients_DetailHistory(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := adminClientsTestRouter(t)
	seed := seedAdminClientsData(t, db)

	centerOneToken := roleToken(t, 7, 1, "beauty_center")
	centerTwoToken := roleToken(t, 8, 2, "beauty_center")

	// ── Client-scoped history, reverse chronological order ──
	// Client 1 at center 1: 08-15 14:00, 08-15 09:00, 08-10 09:00.
	w := do(r, http.MethodGet, "/api/reservation/center/1?clientId=1", centerOneToken, "")
	assert.Equal(t, http.StatusOK, w.Code)
	var list []adminReservationLite
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	if assert.Len(t, list, 3) {
		assert.Equal(t, []uint{seed.c1DayB14.ID, seed.c1DayB09.ID, seed.c1DayA.ID},
			[]uint{list[0].ID, list[1].ID, list[2].ID}, "date DESC, start_time DESC")
		for _, res := range list {
			assert.Equal(t, uint(1), res.BeautyCenterID, "no cross-center leakage")
			assert.Equal(t, uint(1), res.ClientID, "only the queried client's reservations")
		}
	}

	// Client 2: its single CANCELLED reservation is still part of the history.
	w = do(r, http.MethodGet, "/api/reservation/center/1?clientId=2", centerOneToken, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	if assert.Len(t, list, 1) {
		assert.Equal(t, seed.c2Cancelled.ID, list[0].ID)
	}

	// Client 5 has no reservations at center 1 → 200 with an empty array.
	w = do(r, http.MethodGet, "/api/reservation/center/1?clientId=5", centerOneToken, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `[]`, "empty result must serialize as [] — never null")

	// ── Cross-center isolation: center 2's token cannot read center 1's data ──
	w = do(r, http.MethodGet, "/api/reservation/center/1?clientId=1", centerTwoToken, "")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "forbidden", errBody(t, w.Body.String()))

	// ── Malformed clientId → 400 ──
	for _, bad := range []string{"abc", "0", "-1", "1.5"} {
		w = do(r, http.MethodGet, "/api/reservation/center/1?clientId="+bad, centerOneToken, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, "clientId=%s must be rejected", bad)
		assert.Equal(t, "bad request", errBody(t, w.Body.String()))
	}

	// ── No clientId → unchanged behavior (regression guards) ──
	// Plain center list: all 5 center-1 reservations, no center-2 leak.
	w = do(r, http.MethodGet, "/api/reservation/center/1", centerOneToken, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	assert.Len(t, list, 5)
	for _, res := range list {
		assert.Equal(t, uint(1), res.BeautyCenterID)
	}

	// Date scoping still works alongside the optional param (A2 behavior kept).
	w = do(r, http.MethodGet, "/api/reservation/center/1?date=2026-08-15", centerOneToken, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	if assert.Len(t, list, 2) {
		assert.Equal(t, []uint{seed.c1DayB09.ID, seed.c1DayB14.ID},
			[]uint{list[0].ID, list[1].ID}, "date-scoped rows keep start_time ASC")
	}
}
