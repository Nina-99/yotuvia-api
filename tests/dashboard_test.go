package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/router"
	"github.com/Nina-99/yottu-api/service"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  Helpers (shared do/roleToken/errBody come from router_authz_test.go)
// ──────────────────────────────────────────────

// utcMidnight returns midnight UTC at the given day offset from today. Seeds
// and service boundaries are both computed in UTC (design decision D2), so
// seeded dates must be UTC midnights to avoid timezone-skewed membership.
func utcMidnight(daysOffset int) time.Time {
	now := time.Now().UTC()
	y, m, d := now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).AddDate(0, 0, daysOffset)
}

// timeDateOnly strips a time.Time to midnight UTC (date column semantics).
func timeDateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// startOfWeekUTC returns Monday 00:00 UTC for the week containing t.
func startOfWeekUTC(t time.Time) time.Time {
	t = timeDateOnly(t)
	offset := (int(t.Weekday()) + 6) % 7 // Monday = 0
	return t.AddDate(0, 0, -offset)
}

// dashboardTestRouter mounts DashboardRoutes exactly as router.go does, on the
// same model set the production AutoMigrate uses.
func dashboardTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	validate := validator.New()
	r := gin.New()
	rg := r.Group("/api")
	router.DashboardRoutes(rg, db, validate)
	return r, db
}

func seedDashboardCenter(t *testing.T, db *gorm.DB, id uint, name, email string, isActive bool) *models.BeautyCenter {
	t.Helper()
	center := &models.BeautyCenter{
		ID:           id,
		Name:         name,
		Email:        email,
		Phone:        fmt.Sprintf("555%04d", id),
		IsActive:     isActive,
		ReferralCode: fmt.Sprintf("REF%03d", id),
	}
	if err := db.Create(center).Error; err != nil {
		t.Fatalf("failed to seed center: %v", err)
	}
	// gorm:"default:true" would coerce zero-value IsActive back to true on
	// insert, so force the explicit value for inactive seeds.
	if !isActive {
		if err := db.Model(center).Update("is_active", false).Error; err != nil {
			t.Fatalf("failed to force center inactive: %v", err)
		}
	}
	return center
}

func seedDashboardClient(t *testing.T, db *gorm.DB, id uint, first, last string) *models.Client {
	t.Helper()
	client := &models.Client{
		Model:     gorm.Model{ID: id},
		FirstName: first,
		LastName:  last,
		Username:  fmt.Sprintf("dashuser%d", id),
		Phone:     fmt.Sprintf("5551%03d", id),
		Email:     fmt.Sprintf("dashuser%d@test.com", id),
		Password:  "hashed",
		IsActive:  true,
	}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("failed to seed client %d: %v", id, err)
	}
	return client
}

func seedDashboardPersonnel(t *testing.T, db *gorm.DB, id, centerID uint, isActive bool) *models.Personnel {
	t.Helper()
	p := &models.Personnel{
		Model:          gorm.Model{ID: id},
		BeautyCenterID: centerID,
		FirstName:      "Staff",
		LastName:       fmt.Sprintf("%d", id),
		Email:          fmt.Sprintf("dashstaff%d@test.com", id),
		Phone:          fmt.Sprintf("5552%03d", id),
		IsActive:       isActive,
	}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("failed to seed personnel %d: %v", id, err)
	}
	return p
}

// seedDashboardService creates a service with an explicit ID (tests reference
// service IDs in reservation plans and assertions).
func seedDashboardService(t *testing.T, db *gorm.DB, id, centerID uint, name string, isActive bool) *models.Service {
	t.Helper()
	svc := &models.Service{
		ID:             id,
		BeautyCenterID: centerID,
		CategoryID:     1,
		Name:           name,
		Description:    "desc " + name,
		Duration:       30,
		Price:          50,
		IsActive:       isActive,
	}
	if err := db.Create(svc).Error; err != nil {
		t.Fatalf("failed to seed service %s: %v", name, err)
	}
	// Same GORM default:true coercion as centers — force inactive explicitly.
	if !isActive {
		if err := db.Model(svc).Update("is_active", false).Error; err != nil {
			t.Fatalf("failed to force service inactive: %v", err)
		}
	}
	return svc
}

func seedDashboardReview(t *testing.T, db *gorm.DB, centerID, clientID uint, rating int) *models.Review {
	t.Helper()
	review := &models.Review{
		BeautyCenterID: centerID,
		ClientID:       clientID,
		Rating:         rating,
		Comment:        "ok",
	}
	if err := db.Create(review).Error; err != nil {
		t.Fatalf("failed to seed review: %v", err)
	}
	return review
}

// seedReservationPlan describes one reservation row plus linked services.
type seedReservationPlan struct {
	date        time.Time
	startTime   string
	endTime     string
	status      string
	totalPrice  float64
	clientID    uint
	personnelID uint
	serviceIDs  []uint
}

func seedReservationWithServices(t *testing.T, db *gorm.DB, centerID uint, plan seedReservationPlan) *models.Reservation {
	t.Helper()
	res := &models.Reservation{
		BeautyCenterID: centerID,
		ClientID:       plan.clientID,
		PersonnelID:    plan.personnelID,
		Date:           plan.date,
		StartTime:      plan.startTime,
		EndTime:        plan.endTime,
		TotalPrice:     plan.totalPrice,
		Status:         plan.status,
	}
	if err := db.Create(res).Error; err != nil {
		t.Fatalf("failed to seed reservation: %v", err)
	}
	for _, sid := range plan.serviceIDs {
		var svc models.Service
		if err := db.First(&svc, sid).Error; err != nil {
			t.Fatalf("failed to load service %d: %v", sid, err)
		}
		rs := &models.ReservationService{
			ReservationID: res.ID,
			ServiceID:     sid,
			ServiceName:   svc.Name,
			Price:         svc.Price,
			Duration:      svc.Duration,
		}
		if err := db.Create(rs).Error; err != nil {
			t.Fatalf("failed to seed reservation service: %v", err)
		}
	}
	return res
}

// revenueSeed tracks a COMPLETED reservation so expected day/week/month revenue
// can be derived with the same UTC boundaries the service uses (D2), keeping
// the assertions run-date independent.
type revenueSeed struct {
	date  time.Time
	total float64
}

// ──────────────────────────────────────────────
//  Happy path — every metric asserted
// ──────────────────────────────────────────────

func TestDashboardStats_HappyPath(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := dashboardTestRouter(t)

	seedDashboardCenter(t, db, 1, "Salon One", "one@test.com", true)
	seedDashboardClient(t, db, 1, "Nina", "Novak")
	seedDashboardClient(t, db, 2, "Beta", "Bell")
	seedDashboardClient(t, db, 3, "Gamma", "Gomez")
	seedDashboardClient(t, db, 4, "Delta", "Diaz")
	seedDashboardClient(t, db, 5, "Epsilon", "Estevez")

	// Services: 3 active + 1 inactive → total_services = 3
	seedDashboardService(t, db, 1, 1, "Corte", true)
	seedDashboardService(t, db, 2, 1, "Color", true)
	seedDashboardService(t, db, 3, 1, "Peinado", true)
	seedDashboardService(t, db, 4, 1, "Inactive Svc", false)

	// Personnel: 2 active + 1 inactive → total_personnel = 2
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardPersonnel(t, db, 3, 1, false)

	// Reviews: count 2, avg (4+5)/2 = 4.5
	seedDashboardReview(t, db, 1, 1, 4)
	seedDashboardReview(t, db, 1, 2, 5)

	completed := []revenueSeed{}
	seed := func(plan seedReservationPlan) *models.Reservation {
		res := seedReservationWithServices(t, db, 1, plan)
		if plan.status == "COMPLETED" {
			completed = append(completed, revenueSeed{date: plan.date, total: plan.totalPrice})
		}
		return res
	}

	today1 := seed(seedReservationPlan{date: utcMidnight(0), startTime: "09:00", endTime: "10:00", status: "COMPLETED", totalPrice: 100, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	today2 := seed(seedReservationPlan{date: utcMidnight(0), startTime: "14:00", endTime: "15:00", status: "PENDING", totalPrice: 80, clientID: 1, personnelID: 1, serviceIDs: []uint{2}})
	cancelledToday := seed(seedReservationPlan{date: utcMidnight(0), startTime: "16:00", endTime: "17:00", status: "CANCELLED", totalPrice: 999, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	seed(seedReservationPlan{date: utcMidnight(-1), startTime: "10:00", endTime: "11:00", status: "COMPLETED", totalPrice: 120, clientID: 1, personnelID: 1, serviceIDs: []uint{1, 2}})
	seed(seedReservationPlan{date: utcMidnight(-7), startTime: "11:00", endTime: "12:00", status: "COMPLETED", totalPrice: 200, clientID: 1, personnelID: 1, serviceIDs: []uint{3}})
	seed(seedReservationPlan{date: utcMidnight(-20), startTime: "12:00", endTime: "13:00", status: "COMPLETED", totalPrice: 50, clientID: 1, personnelID: 1, serviceIDs: []uint{2}})
	u1 := seed(seedReservationPlan{date: utcMidnight(1), startTime: "10:00", endTime: "11:00", status: "CONFIRMED", totalPrice: 60, clientID: 3, personnelID: 1, serviceIDs: []uint{2}})
	u2 := seed(seedReservationPlan{date: utcMidnight(1), startTime: "09:00", endTime: "10:00", status: "PENDING", totalPrice: 70, clientID: 2, personnelID: 1, serviceIDs: []uint{1}})
	u3 := seed(seedReservationPlan{date: utcMidnight(2), startTime: "08:00", endTime: "09:00", status: "PENDING", totalPrice: 90, clientID: 4, personnelID: 1, serviceIDs: []uint{3}})
	u4 := seed(seedReservationPlan{date: utcMidnight(4), startTime: "09:30", endTime: "10:30", status: "CONFIRMED", totalPrice: 110, clientID: 4, personnelID: 1, serviceIDs: []uint{2}})
	u5 := seed(seedReservationPlan{date: utcMidnight(6), startTime: "12:00", endTime: "13:00", status: "PENDING", totalPrice: 130, clientID: 5, personnelID: 1, serviceIDs: []uint{1}})

	w := do(r, http.MethodGet, "/api/dashboard/stats", roleToken(t, 7, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusOK, w.Code)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}

	var got response.DashboardStatsResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))

	// KPI metrics
	assert.Equal(t, 2, got.AppointmentsToday, "today1+today2; cancelled excluded")
	assert.Equal(t, 4, got.PendingAppointments, "today2, u2, u3, u5")
	assert.Equal(t, 3, got.TotalServices, "inactive service excluded")
	assert.Equal(t, 2, got.TotalPersonnel, "inactive staff excluded")
	assert.Equal(t, 2, got.TotalReviews)
	assert.Equal(t, 4.5, got.AverageRating, "(4+5)/2")

	// Revenue — today is deterministic; week/month derive with the UTC
	// boundaries the service defines (D2).
	assert.Equal(t, 100.0, got.RevenueToday, "only today1 is COMPLETED today; cancelled 999 ignored")
	now := time.Now().UTC()
	weekStart := startOfWeekUTC(now)
	weekEnd := weekStart.AddDate(0, 0, 7)
	y, m, _ := now.Date()
	monthStart := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)
	var expectedWeek, expectedMonth float64
	for _, cs := range completed {
		if !cs.date.Before(weekStart) && cs.date.Before(weekEnd) {
			expectedWeek += cs.total
		}
		if !cs.date.Before(monthStart) && cs.date.Before(monthEnd) {
			expectedMonth += cs.total
		}
	}
	assert.Equal(t, expectedWeek, got.RevenueWeek)
	assert.Equal(t, expectedMonth, got.RevenueMonth)

	// Average ticket = all COMPLETED revenue / all COMPLETED count
	var expectedTicket, completedCount float64
	for _, cs := range completed {
		expectedTicket += cs.total
		completedCount++
	}
	assert.Equal(t, expectedTicket/completedCount, got.AverageTicket)
	assert.Equal(t, 117.5, got.AverageTicket, "(100+120+200+50)/4")

	// Top services (feat-admin-analytics D5): range-scoped leaderboard ranked
	// by SUM(reservation_services.price) over COMPLETED only. Seeds copy each
	// service's Price (50) into reservation rows, so revenue follows completed
	// usage within [today-29, today]: S1 = today1 + yesterday = 100; S2 =
	// yesterday + -20d = 100; S3 = -7d = 50. Ties broken by distinct
	// appointment count then id → S1, S2, S3.
	assert.Len(t, got.TopServices, 3)
	if assert.Len(t, got.TopServices, 3) {
		assert.Equal(t, []uint{1, 2, 3}, []uint{
			got.TopServices[0].ServiceID, got.TopServices[1].ServiceID, got.TopServices[2].ServiceID,
		}, "ranked by revenue DESC with stable tie-breaks")
		assert.Equal(t, []float64{100, 100, 50}, []float64{
			got.TopServices[0].Revenue, got.TopServices[1].Revenue, got.TopServices[2].Revenue,
		})
		assert.Equal(t, []int{2, 2, 1}, []int{
			got.TopServices[0].AppointmentCount, got.TopServices[1].AppointmentCount, got.TopServices[2].AppointmentCount,
		}, "distinct COMPLETED reservations per service in range")
	}

	// Upcoming: next 3 by (date, start_time) among PENDING/CONFIRMED >= today:
	// today2 (today 14:00) → u2 (t+1 09:00) → u1 (t+1 10:00). u3/u4/u5 dropped.
	if assert.Len(t, got.UpcomingReservations, 3) {
		ids := []uint{got.UpcomingReservations[0].ID, got.UpcomingReservations[1].ID, got.UpcomingReservations[2].ID}
		assert.Equal(t, []uint{today2.ID, u2.ID, u1.ID}, ids, "sorted by date then start_time")
		assert.Equal(t, []string{"Nina Novak", "Beta Bell", "Gamma Gomez"}, []string{
			got.UpcomingReservations[0].ClientName,
			got.UpcomingReservations[1].ClientName,
			got.UpcomingReservations[2].ClientName,
		})
		assert.Equal(t, []string{"14:00", "09:00", "10:00"}, []string{
			got.UpcomingReservations[0].StartTime,
			got.UpcomingReservations[1].StartTime,
			got.UpcomingReservations[2].StartTime,
		})
		assert.Equal(t, utcMidnight(0).Format("2006-01-02"), got.UpcomingReservations[0].Date)
		assert.Equal(t, "PENDING", got.UpcomingReservations[0].Status)
		assert.Equal(t, 80.0, got.UpcomingReservations[0].TotalPrice)
	}

	// The CANCELLED reservation never leaks: not in today's count or revenue.
	assert.NotEqual(t, 999.0, got.RevenueToday)
	_ = cancelledToday
	_ = u3
	_ = u4
	_ = u5
	_ = today1
}

// ──────────────────────────────────────────────
//  Empty database → zeros and [] (never null)
// ──────────────────────────────────────────────

func TestDashboardStats_EmptyDatabase(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := dashboardTestRouter(t)
	seedDashboardCenter(t, db, 1, "Empty Salon", "empty@test.com", true)

	w := do(r, http.MethodGet, "/api/dashboard/stats", roleToken(t, 7, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusOK, w.Code)

	var got response.DashboardStatsResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, 0, got.AppointmentsToday)
	assert.Equal(t, 0, got.PendingAppointments)
	assert.Equal(t, 0, got.TotalServices)
	assert.Equal(t, 0, got.TotalPersonnel)
	assert.Equal(t, 0, got.TotalReviews)
	assert.Equal(t, 0.0, got.AverageRating)
	assert.Equal(t, 0.0, got.RevenueToday)
	assert.Equal(t, 0.0, got.RevenueWeek)
	assert.Equal(t, 0.0, got.RevenueMonth)
	assert.Equal(t, 0.0, got.AverageTicket)
	assert.Empty(t, got.TopServices)
	assert.Empty(t, got.UpcomingReservations)

	// ── Phase-2 wire contract (feat-admin-analytics) ──
	// Deltas: no baseline in the previous window → null.
	assert.Nil(t, got.RevenueDeltaPct)
	assert.Nil(t, got.AppointmentsDeltaPct)
	// Rates: zero-safe.
	assert.Equal(t, 0, got.CancelledCount)
	assert.Equal(t, 0, got.TotalAppointments)
	assert.Equal(t, 0.0, got.CancellationRatePct)
	// Heatmap: full 7×24 zeros — never null.
	if assert.Len(t, got.HourHeatmap, 7) {
		for d, row := range got.HourHeatmap {
			assert.Len(t, row, 24, "day %d must have 24 hour cells", d)
			for h, v := range row {
				assert.Equal(t, 0, v, "cell [%d][%d] must be 0", d, h)
			}
		}
	}
	assert.Empty(t, got.PersonnelRevenue)

	// Wire contract: arrays serialize as [] / null deltas — never missing.
	body := w.Body.String()
	assert.Contains(t, body, `"top_services":[]`)
	assert.Contains(t, body, `"upcoming_reservations":[]`)
	assert.Contains(t, body, `"personnel_revenue":[]`)
	assert.Contains(t, body, `"revenue_delta_pct":null`)
	assert.Contains(t, body, `"appointments_delta_pct":null`)
	assert.Contains(t, body, `"hour_heatmap":[[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`)
}

// ──────────────────────────────────────────────
//  Truncation — top 3 services, next 3 upcoming
// ──────────────────────────────────────────────

func TestDashboardStats_Truncation(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := dashboardTestRouter(t)

	seedDashboardCenter(t, db, 1, "Trunc Salon", "trunc@test.com", true)
	for i := 1; i <= 5; i++ {
		seedDashboardClient(t, db, uint(i), fmt.Sprintf("Client%d", i), "Last")
		seedDashboardService(t, db, uint(i), 1, fmt.Sprintf("Trunc%d", i), true)
	}
	seedDashboardPersonnel(t, db, 1, 1, true)

	// 5 past COMPLETED reservations with cumulative service counts:
	// S1=5, S2=4, S3=3, S4=2, S5=1 → top 3 must be S1, S2, S3.
	// feat-admin-analytics D5: leaderboard is COMPLETED-only and range-scoped,
	// so the seeds must be COMPLETED (CONFIRMED would no longer rank).
	for i := 1; i <= 5; i++ {
		var sids []uint
		for j := 1; j <= i; j++ {
			sids = append(sids, uint(j))
		}
		seedReservationWithServices(t, db, 1, seedReservationPlan{
			date: utcMidnight(-i), startTime: "09:00", endTime: "10:00",
			status: "COMPLETED", totalPrice: 60, clientID: uint(i), personnelID: 1, serviceIDs: sids,
		})
	}

	// 5 upcoming PENDING/CONFIRMED → next 3 by (date, start_time).
	upcomingPlans := []seedReservationPlan{
		{date: utcMidnight(1), startTime: "09:00", endTime: "10:00", status: "PENDING", totalPrice: 70, clientID: 1, personnelID: 1},
		{date: utcMidnight(1), startTime: "10:00", endTime: "11:00", status: "CONFIRMED", totalPrice: 80, clientID: 2, personnelID: 1},
		{date: utcMidnight(2), startTime: "08:00", endTime: "09:00", status: "PENDING", totalPrice: 90, clientID: 3, personnelID: 1},
		{date: utcMidnight(3), startTime: "09:00", endTime: "10:00", status: "CONFIRMED", totalPrice: 100, clientID: 4, personnelID: 1},
		{date: utcMidnight(4), startTime: "10:00", endTime: "11:00", status: "PENDING", totalPrice: 110, clientID: 5, personnelID: 1},
	}
	var seededUpcoming []*models.Reservation
	for _, p := range upcomingPlans {
		seededUpcoming = append(seededUpcoming, seedReservationWithServices(t, db, 1, p))
	}

	w := do(r, http.MethodGet, "/api/dashboard/stats", roleToken(t, 7, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusOK, w.Code)

	var got response.DashboardStatsResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))

	// Top 3 of 5 services, most booked first.
	if assert.Len(t, got.TopServices, 3) {
		assert.Equal(t, []uint{1, 2, 3}, []uint{
			got.TopServices[0].ServiceID, got.TopServices[1].ServiceID, got.TopServices[2].ServiceID,
		})
		assert.Equal(t, []int{5, 4, 3}, []int{
			got.TopServices[0].AppointmentCount, got.TopServices[1].AppointmentCount, got.TopServices[2].AppointmentCount,
		})
	}

	// Next 3 of 5 upcoming, sorted by date then start_time.
	if assert.Len(t, got.UpcomingReservations, 3) {
		want := []uint{seededUpcoming[0].ID, seededUpcoming[1].ID, seededUpcoming[2].ID}
		gotIDs := []uint{
			got.UpcomingReservations[0].ID, got.UpcomingReservations[1].ID, got.UpcomingReservations[2].ID,
		}
		assert.Equal(t, want, gotIDs, "next 3 by (date, start_time)")
		assert.Equal(t, []string{"09:00", "10:00", "08:00"}, []string{
			got.UpcomingReservations[0].StartTime,
			got.UpcomingReservations[1].StartTime,
			got.UpcomingReservations[2].StartTime,
		})
	}
}

// ──────────────────────────────────────────────
//  Authz boundaries — 401/403/404
// ──────────────────────────────────────────────

func TestDashboardStats_Authz(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := dashboardTestRouter(t)
	seedDashboardCenter(t, db, 1, "Salon One", "one@test.com", true)
	seedDashboardCenter(t, db, 2, "Salon Two", "two@test.com", false) // inactive

	path := "/api/dashboard/stats"

	t.Run("no token is 401", func(t *testing.T) {
		w := do(r, http.MethodGet, path, "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, "Authorization header missing", errBody(t, w.Body.String()))
	})

	t.Run("client role is 403", func(t *testing.T) {
		w := do(r, http.MethodGet, path, roleToken(t, 1, 0, "client"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("personnel role is 403", func(t *testing.T) {
		w := do(r, http.MethodGet, path, roleToken(t, 1, 1, "personnel"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("missing centerID claim is 403", func(t *testing.T) {
		w := do(r, http.MethodGet, path, roleToken(t, 1, 0, "beauty_center"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("inactive center is 404", func(t *testing.T) {
		w := do(r, http.MethodGet, path, roleToken(t, 1, 2, "beauty_center"), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "not found", errBody(t, w.Body.String()))
	})
}

// ──────────────────────────────────────────────
//  T5 — Repository Unit Tests: by-day GROUP BY correctness
// ──────────────────────────────────────────────

// TestDashboardRepo_RevenueByDay verifies RevenueByDay returns daily revenue
// sums for COMPLETED reservations only, grouped by date in UTC.
func TestDashboardRepo_RevenueByDay(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)

	seedDashboardCenter(t, db, 1, "Repo Test Center", "repo@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Test Service", true)

	base := utcMidnight(-3)

	// Day 0: 2 COMPLETED = 150, 1 PENDING (ignored), 1 CANCELLED (ignored)
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base, startTime: "09:00", endTime: "10:00", status: "COMPLETED", totalPrice: 100,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base, startTime: "11:00", endTime: "12:00", status: "COMPLETED", totalPrice: 50,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base, startTime: "14:00", endTime: "15:00", status: "PENDING", totalPrice: 80,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base, startTime: "16:00", endTime: "17:00", status: "CANCELLED", totalPrice: 999,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	// Day 1: 1 COMPLETED = 200
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base.AddDate(0, 0, 1), startTime: "10:00", endTime: "11:00", status: "COMPLETED", totalPrice: 200,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	// Day 2: no COMPLETED (only PENDING) → should not appear in results
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base.AddDate(0, 0, 2), startTime: "09:00", endTime: "10:00", status: "PENDING", totalPrice: 75,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	from := base
	to := base.AddDate(0, 0, 3) // half-open [from, to)

	rows, err := repo.RevenueByDay(1, from, to)
	assert.NoError(t, err)
	assert.Len(t, rows, 2, "only days with COMPLETED reservations appear")

	// Assert day 0: 100 + 50 = 150
	assert.Equal(t, base.Format("2006-01-02"), rows[0].Date.Format("2006-01-02"))
	assert.Equal(t, 150.0, rows[0].Value)

	// Assert day 1: 200
	assert.Equal(t, base.AddDate(0, 0, 1).Format("2006-01-02"), rows[1].Date.Format("2006-01-02"))
	assert.Equal(t, 200.0, rows[1].Value)
}

// TestDashboardRepo_AppointmentsByDay verifies AppointmentsByDay returns daily
// counts for non-CANCELLED reservations, grouped by date in UTC.
func TestDashboardRepo_AppointmentsByDay(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)

	seedDashboardCenter(t, db, 1, "Repo Test Center", "repo@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Test Service", true)

	base := utcMidnight(-3)

	// Day 0: COMPLETED + PENDING = 2 (CANCELLED excluded)
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base, startTime: "09:00", endTime: "10:00", status: "COMPLETED", totalPrice: 100,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base, startTime: "11:00", endTime: "12:00", status: "PENDING", totalPrice: 80,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base, startTime: "14:00", endTime: "15:00", status: "CANCELLED", totalPrice: 999,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	// Day 1: CONFIRMED + PENDING = 2
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base.AddDate(0, 0, 1), startTime: "10:00", endTime: "11:00", status: "CONFIRMED", totalPrice: 200,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base.AddDate(0, 0, 1), startTime: "14:00", endTime: "15:00", status: "PENDING", totalPrice: 150,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	// Day 2: only CANCELLED → should not appear
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base.AddDate(0, 0, 2), startTime: "09:00", endTime: "10:00", status: "CANCELLED", totalPrice: 75,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	from := base
	to := base.AddDate(0, 0, 3) // half-open [from, to)

	rows, err := repo.AppointmentsByDay(1, from, to)
	assert.NoError(t, err)
	assert.Len(t, rows, 2, "only days with non-CANCELLED reservations appear")

	// Assert day 0: 2 appointments (COMPLETED + PENDING)
	assert.Equal(t, base.Format("2006-01-02"), rows[0].Date.Format("2006-01-02"))
	assert.Equal(t, int64(2), rows[0].Value)

	// Assert day 1: 2 appointments (CONFIRMED + PENDING)
	assert.Equal(t, base.AddDate(0, 0, 1).Format("2006-01-02"), rows[1].Date.Format("2006-01-02"))
	assert.Equal(t, int64(2), rows[1].Value)
}

// ──────────────────────────────────────────────
//  T6 — Service Unit Tests: zero-fill, UTC boundaries
// ──────────────────────────────────────────────

// TestDashboardService_GetStatsWithRange_ZeroFill verifies that missing days
// in the range are zero-filled in both revenue and appointments series.
func TestDashboardService_GetStatsWithRange_ZeroFill(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)
	svc := service.NewDashboardServiceImpl(repo, nil)

	seedDashboardCenter(t, db, 1, "Service Test Center", "svc@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Test Service", true)

	// Only seed data for day 0 and day 2; day 1 should be zero-filled
	base := utcMidnight(-5)

	// Day 0: COMPLETED revenue 100, 1 non-cancelled appt
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base, startTime: "09:00", endTime: "10:00", status: "COMPLETED", totalPrice: 100,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	// Day 2: COMPLETED revenue 200, 2 non-cancelled appts
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base.AddDate(0, 0, 2), startTime: "10:00", endTime: "11:00", status: "COMPLETED", totalPrice: 200,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base.AddDate(0, 0, 2), startTime: "14:00", endTime: "15:00", status: "PENDING", totalPrice: 150,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	// Request range covering 3 days: day 0, 1, 2
	from := base
	to := base.AddDate(0, 0, 2) // inclusive end; service treats as half-open [from, to+1)

	resp, err := svc.GetStatsWithRange(1, from, to, 30)
	assert.NoError(t, err)

	// Should have 3 days in each series (day 0, 1, 2)
	assert.Len(t, resp.RevenueSeries, 3)
	assert.Len(t, resp.AppointmentsSeries, 3)

	// Day 0: revenue 100, appointments 1
	assert.Equal(t, base.Format("2006-01-02"), resp.RevenueSeries[0].Date)
	assert.Equal(t, 100.0, resp.RevenueSeries[0].Value)
	assert.Equal(t, base.Format("2006-01-02"), resp.AppointmentsSeries[0].Date)
	assert.Equal(t, 1.0, resp.AppointmentsSeries[0].Value)

	// Day 1: zero-filled
	assert.Equal(t, base.AddDate(0, 0, 1).Format("2006-01-02"), resp.RevenueSeries[1].Date)
	assert.Equal(t, 0.0, resp.RevenueSeries[1].Value)
	assert.Equal(t, base.AddDate(0, 0, 1).Format("2006-01-02"), resp.AppointmentsSeries[1].Date)
	assert.Equal(t, 0.0, resp.AppointmentsSeries[1].Value)

	// Day 2: revenue 200, appointments 2
	assert.Equal(t, base.AddDate(0, 0, 2).Format("2006-01-02"), resp.RevenueSeries[2].Date)
	assert.Equal(t, 200.0, resp.RevenueSeries[2].Value)
	assert.Equal(t, base.AddDate(0, 0, 2).Format("2006-01-02"), resp.AppointmentsSeries[2].Date)
	assert.Equal(t, 2.0, resp.AppointmentsSeries[2].Value)
}

// TestDashboardService_GetStatsWithRange_RangeDefaults verifies that when the
// controller defaults are applied (handled at HTTP layer), the service correctly
// handles the provided range. This test focuses on the service behavior with
// an explicit 30-day range.
func TestDashboardService_GetStatsWithRange_RangeDefaults(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)
	svc := service.NewDashboardServiceImpl(repo, nil)

	seedDashboardCenter(t, db, 1, "Service Test Center", "svc@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Test Service", true)

	// Seed one reservation 15 days ago
	base := utcMidnight(-15)
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base, startTime: "10:00", endTime: "11:00", status: "COMPLETED", totalPrice: 300,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	// 30-day range ending today
	from := utcMidnight(-29)
	to := utcMidnight(0)

	resp, err := svc.GetStatsWithRange(1, from, to, 30)
	assert.NoError(t, err)

	// Should have exactly 30 days (half-open [from, to+1) = 30 days)
	assert.Len(t, resp.RevenueSeries, 30)
	assert.Len(t, resp.AppointmentsSeries, 30)

	// Day at index 14 (15 days ago) should have the data
	assert.Equal(t, base.Format("2006-01-02"), resp.RevenueSeries[14].Date)
	assert.Equal(t, 300.0, resp.RevenueSeries[14].Value)
	assert.Equal(t, 1.0, resp.AppointmentsSeries[14].Value)

	// All other days should be zero
	for i := 0; i < 30; i++ {
		if i != 14 {
			assert.Equal(t, 0.0, resp.RevenueSeries[i].Value, "day %d should be zero", i)
			assert.Equal(t, 0.0, resp.AppointmentsSeries[i].Value, "day %d should be zero", i)
		}
	}
}

// TestDashboardService_GetStatsWithRange_FutureToAllowed verifies that a `to`
// date in the future is allowed and future days are zero-filled.
func TestDashboardService_GetStatsWithRange_FutureToAllowed(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)
	svc := service.NewDashboardServiceImpl(repo, nil)

	seedDashboardCenter(t, db, 1, "Service Test Center", "svc@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Test Service", true)

	// Seed data for today
	today := utcMidnight(0)
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: today, startTime: "10:00", endTime: "11:00", status: "COMPLETED", totalPrice: 150,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	// Range: today through 5 days in the future
	from := today
	to := today.AddDate(0, 0, 5) // inclusive end → 6 days total

	resp, err := svc.GetStatsWithRange(1, from, to, 30)
	assert.NoError(t, err)

	// Should have 6 days (today + 5 future)
	assert.Len(t, resp.RevenueSeries, 6)
	assert.Len(t, resp.AppointmentsSeries, 6)

	// Today: revenue 150, 1 appointment
	assert.Equal(t, today.Format("2006-01-02"), resp.RevenueSeries[0].Date)
	assert.Equal(t, 150.0, resp.RevenueSeries[0].Value)
	assert.Equal(t, 1.0, resp.AppointmentsSeries[0].Value)

	// Future days: all zero-filled
	for i := 1; i < 6; i++ {
		assert.Equal(t, 0.0, resp.RevenueSeries[i].Value, "future day %d should be zero", i)
		assert.Equal(t, 0.0, resp.AppointmentsSeries[i].Value, "future day %d should be zero", i)
	}
}

// ──────────────────────────────────────────────
//  T7 — HTTP Integration Tests: authz + params + series contract
// ──────────────────────────────────────────────

// TestDashboardStats_TimeSeries_HappyPath covers the happy path with data
// spanning the range, verifying series arrays present with correct sums.
func TestDashboardStats_TimeSeries_HappyPath(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := dashboardTestRouter(t)

	seedDashboardCenter(t, db, 1, "Series Happy Center", "happy@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardClient(t, db, 2, "Client", "Two")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Service A", true)
	seedDashboardService(t, db, 2, 1, "Service B", true)

	base := utcMidnight(-2)

	// Day 0 (base): 2 COMPLETED = 150, 1 PENDING, 1 CANCELLED
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base, startTime: "09:00", endTime: "10:00", status: "COMPLETED", totalPrice: 100,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base, startTime: "11:00", endTime: "12:00", status: "COMPLETED", totalPrice: 50,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base, startTime: "14:00", endTime: "15:00", status: "PENDING", totalPrice: 80,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base, startTime: "16:00", endTime: "17:00", status: "CANCELLED", totalPrice: 999,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	// Day 1 (base+1): 1 COMPLETED = 200, 1 PENDING
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base.AddDate(0, 0, 1), startTime: "10:00", endTime: "11:00", status: "COMPLETED", totalPrice: 200,
		clientID: 1, personnelID: 1, serviceIDs: []uint{2},
	})
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base.AddDate(0, 0, 1), startTime: "14:00", endTime: "15:00", status: "PENDING", totalPrice: 150,
		clientID: 2, personnelID: 1, serviceIDs: []uint{1},
	})

	// Day 2 (base+2): no COMPLETED, 1 CONFIRMED
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base.AddDate(0, 0, 2), startTime: "09:00", endTime: "10:00", status: "CONFIRMED", totalPrice: 75,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	fromStr := base.Format("2006-01-02")
	toStr := base.AddDate(0, 0, 2).Format("2006-01-02")
	path := "/api/dashboard/stats?from=" + fromStr + "&to=" + toStr

	w := do(r, http.MethodGet, path, roleToken(t, 7, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusOK, w.Code)

	var got response.DashboardStatsResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))

	// 3 days in range [from, to] inclusive → half-open [from, to+1) = 3 days
	assert.Len(t, got.RevenueSeries, 3)
	assert.Len(t, got.AppointmentsSeries, 3)

	// Revenue: day0=150 (COMPLETED only), day1=200, day2=0 (no COMPLETED)
	assert.Equal(t, base.Format("2006-01-02"), got.RevenueSeries[0].Date)
	assert.Equal(t, 150.0, got.RevenueSeries[0].Value)
	assert.Equal(t, base.AddDate(0, 0, 1).Format("2006-01-02"), got.RevenueSeries[1].Date)
	assert.Equal(t, 200.0, got.RevenueSeries[1].Value)
	assert.Equal(t, base.AddDate(0, 0, 2).Format("2006-01-02"), got.RevenueSeries[2].Date)
	assert.Equal(t, 0.0, got.RevenueSeries[2].Value)

	// Appointments: day0=3 (2 COMPLETED+1 PENDING), day1=2 (COMPLETED+PENDING), day2=1 (CONFIRMED)
	assert.Equal(t, base.Format("2006-01-02"), got.AppointmentsSeries[0].Date)
	assert.Equal(t, 3.0, got.AppointmentsSeries[0].Value)
	assert.Equal(t, base.AddDate(0, 0, 1).Format("2006-01-02"), got.AppointmentsSeries[1].Date)
	assert.Equal(t, 2.0, got.AppointmentsSeries[1].Value)
	assert.Equal(t, base.AddDate(0, 0, 2).Format("2006-01-02"), got.AppointmentsSeries[2].Date)
	assert.Equal(t, 1.0, got.AppointmentsSeries[2].Value)

	// Series never null - arrays present
	assert.NotNil(t, got.RevenueSeries)
	assert.NotNil(t, got.AppointmentsSeries)
}

// TestDashboardStats_TimeSeries_EmptyRange covers zero-filled arrays when
// the range has no data.
func TestDashboardStats_TimeSeries_EmptyRange(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := dashboardTestRouter(t)

	seedDashboardCenter(t, db, 1, "Empty Range Center", "empty@test.com", true)

	fromStr := "2026-08-01"
	toStr := "2026-08-03"
	path := "/api/dashboard/stats?from=" + fromStr + "&to=" + toStr

	w := do(r, http.MethodGet, path, roleToken(t, 7, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusOK, w.Code)

	var got response.DashboardStatsResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))

	// 3 days: 2026-08-01, 2026-08-02, 2026-08-03
	assert.Len(t, got.RevenueSeries, 3)
	assert.Len(t, got.AppointmentsSeries, 3)

	for i := 0; i < 3; i++ {
		assert.Equal(t, 0.0, got.RevenueSeries[i].Value)
		assert.Equal(t, 0.0, got.AppointmentsSeries[i].Value)
	}

	// Series present as empty arrays, not null
	assert.Contains(t, w.Body.String(), `"revenue_series":[`)
	assert.Contains(t, w.Body.String(), `"appointments_series":[`)
}

// TestDashboardStats_TimeSeries_AbsentParams covers the default 30-day window
// when no from/to params are provided.
func TestDashboardStats_TimeSeries_AbsentParams(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := dashboardTestRouter(t)

	seedDashboardCenter(t, db, 1, "Default Range Center", "default@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Test Service", true)

	// Seed data 15 days ago
	fifteenDaysAgo := utcMidnight(-15)
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: fifteenDaysAgo, startTime: "10:00", endTime: "11:00", status: "COMPLETED", totalPrice: 250,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	// No from/to params
	w := do(r, http.MethodGet, "/api/dashboard/stats", roleToken(t, 7, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusOK, w.Code)

	var got response.DashboardStatsResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))

	// Default 30 days: [today-29, today] inclusive = 30 days
	assert.Len(t, got.RevenueSeries, 30)
	assert.Len(t, got.AppointmentsSeries, 30)

	// The seeded day should be at index 14 (15 days ago from today = today-15)
	// Range is [today-29, today] → index 0 = today-29, index 14 = today-15
	assert.Equal(t, fifteenDaysAgo.Format("2006-01-02"), got.RevenueSeries[14].Date)
	assert.Equal(t, 250.0, got.RevenueSeries[14].Value)
	assert.Equal(t, 1.0, got.AppointmentsSeries[14].Value)
}

// TestDashboardStats_TimeSeries_OnlyFrom covers the case where only `from`
// is provided, defaulting `to` to `from + 30 days`.
func TestDashboardStats_TimeSeries_OnlyFrom(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := dashboardTestRouter(t)

	seedDashboardCenter(t, db, 1, "Only From Center", "onlyfrom@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Test Service", true)

	fromDate := utcMidnight(-10)
	fromStr := fromDate.Format("2006-01-02")
	path := "/api/dashboard/stats?from=" + fromStr

	w := do(r, http.MethodGet, path, roleToken(t, 7, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusOK, w.Code)

	var got response.DashboardStatsResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))

	// 31 days: from through from+30 inclusive
	assert.Len(t, got.RevenueSeries, 31)
	assert.Len(t, got.AppointmentsSeries, 31)

	// First day should be the from date
	assert.Equal(t, fromStr, got.RevenueSeries[0].Date)
	assert.Equal(t, fromStr, got.AppointmentsSeries[0].Date)

	// Last day should be from + 30
	lastDate := fromDate.AddDate(0, 0, 30).Format("2006-01-02")
	assert.Equal(t, lastDate, got.RevenueSeries[30].Date)
	assert.Equal(t, lastDate, got.AppointmentsSeries[30].Date)
}

// TestDashboardStats_TimeSeries_FromAfterTo covers the validation error when
// from > to.
func TestDashboardStats_TimeSeries_FromAfterTo(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := dashboardTestRouter(t)

	seedDashboardCenter(t, db, 1, "Validation Center", "valid@test.com", true)

	fromStr := "2026-08-15"
	toStr := "2026-08-10"
	path := "/api/dashboard/stats?from=" + fromStr + "&to=" + toStr

	w := do(r, http.MethodGet, path, roleToken(t, 7, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Verify api-error-contract shape
	assert.Contains(t, w.Body.String(), `"error"`)
}

// TestDashboardStats_TimeSeries_MalformedDate covers the validation error
// when date strings are malformed.
func TestDashboardStats_TimeSeries_MalformedDate(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := dashboardTestRouter(t)

	seedDashboardCenter(t, db, 1, "Validation Center", "valid@test.com", true)

	// Malformed date format
	path := "/api/dashboard/stats?from=2026/08/10&to=2026-08-15"

	w := do(r, http.MethodGet, path, roleToken(t, 7, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	assert.Contains(t, w.Body.String(), `"error"`)
}

// TestDashboardStats_TimeSeries_FutureTo covers future `to` date being
// allowed and zero-filled.
func TestDashboardStats_TimeSeries_FutureTo(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := dashboardTestRouter(t)

	seedDashboardCenter(t, db, 1, "Future To Center", "future@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Test Service", true)

	// Seed data for today
	today := utcMidnight(0)
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: today, startTime: "10:00", endTime: "11:00", status: "COMPLETED", totalPrice: 120,
		clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	fromStr := today.Format("2006-01-02")
	toStr := today.AddDate(0, 0, 5).Format("2006-01-02") // 5 days in future
	path := "/api/dashboard/stats?from=" + fromStr + "&to=" + toStr

	w := do(r, http.MethodGet, path, roleToken(t, 7, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusOK, w.Code)

	var got response.DashboardStatsResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))

	// 6 days: today through today+5
	assert.Len(t, got.RevenueSeries, 6)
	assert.Len(t, got.AppointmentsSeries, 6)

	// Today has data
	assert.Equal(t, 120.0, got.RevenueSeries[0].Value)
	assert.Equal(t, 1.0, got.AppointmentsSeries[0].Value)

	// Future days zero-filled
	for i := 1; i < 6; i++ {
		assert.Equal(t, 0.0, got.RevenueSeries[i].Value)
		assert.Equal(t, 0.0, got.AppointmentsSeries[i].Value)
	}
}

// TestDashboardStats_TimeSeries_AuthzMatrix covers the authorization matrix
// with from/to params — should be unchanged from base authz.
func TestDashboardStats_TimeSeries_AuthzMatrix(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := dashboardTestRouter(t)

	seedDashboardCenter(t, db, 1, "Authz Center", "authz@test.com", true)
	seedDashboardCenter(t, db, 2, "Inactive Center", "inactive@test.com", false)

	fromStr := "2026-08-01"
	toStr := "2026-08-03"
	path := "/api/dashboard/stats?from=" + fromStr + "&to=" + toStr

	t.Run("no token is 401", func(t *testing.T) {
		w := do(r, http.MethodGet, path, "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, "Authorization header missing", errBody(t, w.Body.String()))
	})

	t.Run("client role is 403", func(t *testing.T) {
		w := do(r, http.MethodGet, path, roleToken(t, 1, 0, "client"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("personnel role is 403", func(t *testing.T) {
		w := do(r, http.MethodGet, path, roleToken(t, 1, 1, "personnel"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("missing centerID claim is 403", func(t *testing.T) {
		w := do(r, http.MethodGet, path, roleToken(t, 1, 0, "beauty_center"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("inactive center is 404", func(t *testing.T) {
		w := do(r, http.MethodGet, path, roleToken(t, 1, 2, "beauty_center"), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "not found", errBody(t, w.Body.String()))
	})
}

// TestDashboardStats_TimeSeries_OnlyTo covers the case where only `to` is
// provided, defaulting `from` to `to - 30 days`.
func TestDashboardStats_TimeSeries_OnlyTo(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := dashboardTestRouter(t)

	seedDashboardCenter(t, db, 1, "Only To Center", "onlyto@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Test Service", true)

	toDate := utcMidnight(0)
	toStr := toDate.Format("2006-01-02")
	path := "/api/dashboard/stats?to=" + toStr

	w := do(r, http.MethodGet, path, roleToken(t, 7, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusOK, w.Code)

	var got response.DashboardStatsResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))

	// 31 days: to-30 through to inclusive
	assert.Len(t, got.RevenueSeries, 31)
	assert.Len(t, got.AppointmentsSeries, 31)

	// First day should be to - 30
	firstDate := toDate.AddDate(0, 0, -30).Format("2006-01-02")
	assert.Equal(t, firstDate, got.RevenueSeries[0].Date)
	assert.Equal(t, firstDate, got.AppointmentsSeries[0].Date)

	// Last day should be the to date
	assert.Equal(t, toStr, got.RevenueSeries[30].Date)
	assert.Equal(t, toStr, got.AppointmentsSeries[30].Date)
}

// ──────────────────────────────────────────────
//  feat-admin-analytics — Phase 2 metrics
// ──────────────────────────────────────────────

// setDashboardServicePrice overrides a service's price so reservation_service
// rows (which copy Price at booking time) exercise revenue scenarios.
func setDashboardServicePrice(t *testing.T, db *gorm.DB, id uint, price float64) {
	t.Helper()
	if err := db.Model(&models.Service{}).Where("id = ?", id).Update("price", price).Error; err != nil {
		t.Fatalf("failed to set service %d price: %v", id, err)
	}
}

// TestDashboardRepo_HeatmapRows verifies HeatmapRows returns one row per
// COMPLETED reservation in range — PENDING/CANCELLED excluded, no grouping.
func TestDashboardRepo_HeatmapRows(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)

	seedDashboardCenter(t, db, 1, "Heatmap Center", "heat@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Svc", true)

	base := utcMidnight(-2)

	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "09:00", endTime: "10:00", status: "COMPLETED", totalPrice: 100, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "23:45", endTime: "23:59", status: "COMPLETED", totalPrice: 40, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base.AddDate(0, 0, 1), startTime: "00:10", endTime: "01:00", status: "COMPLETED", totalPrice: 30, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "12:00", endTime: "13:00", status: "PENDING", totalPrice: 80, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "14:00", endTime: "15:00", status: "CANCELLED", totalPrice: 999, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})

	rows, err := repo.HeatmapRows(1, base, base.AddDate(0, 0, 2))
	assert.NoError(t, err)
	assert.Len(t, rows, 3, "only COMPLETED reservations feed the heatmap")
	assert.Equal(t, "09:00", rows[0].StartTime)
	assert.Equal(t, "23:45", rows[1].StartTime)
	assert.Equal(t, "00:10", rows[2].StartTime)
}

// TestDashboardRepo_CountStatusBetween verifies the cancellation-rate inputs:
// total counts every status in range; cancelled counts CANCELLED only.
func TestDashboardRepo_CountStatusBetween(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)

	seedDashboardCenter(t, db, 1, "Counts Center", "counts@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Svc", true)

	base := utcMidnight(-1)
	plans := []struct {
		hour   string
		status string
	}{
		{"09:00", "COMPLETED"},
		{"10:00", "COMPLETED"},
		{"11:00", "COMPLETED"},
		{"12:00", "PENDING"},
		{"13:00", "CONFIRMED"},
		{"14:00", "CANCELLED"},
		{"15:00", "CANCELLED"},
	}
	for _, p := range plans {
		seedReservationWithServices(t, db, 1, seedReservationPlan{
			date: base, startTime: p.hour, endTime: "16:00", status: p.status,
			totalPrice: 10, clientID: 1, personnelID: 1, serviceIDs: []uint{1},
		})
	}

	row, err := repo.CountStatusBetween(1, base, base.AddDate(0, 0, 1))
	assert.NoError(t, err)
	assert.Equal(t, int64(7), row.Total)
	assert.Equal(t, int64(2), row.Cancelled)

	// Out-of-range rows don't leak in.
	rowEmpty, err := repo.CountStatusBetween(1, base.AddDate(0, 30, 0), base.AddDate(0, 31, 0))
	assert.NoError(t, err)
	assert.Equal(t, int64(0), rowEmpty.Total)
}

// TestDashboardRepo_TopServicesByRevenue_Flip proves the D5 semantic change:
// an expensive-but-rare service outranks a cheap-but-frequent one, which the
// legacy count-based ranking would have placed first.
func TestDashboardRepo_TopServicesByRevenue_Flip(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)

	seedDashboardCenter(t, db, 1, "Flip Center", "flip@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardPersonnel(t, db, 1, 1, true)
	// Cheap service (id 1, $5) vs expensive service (id 2, $200).
	seedDashboardService(t, db, 1, 1, "Cheap", true)
	setDashboardServicePrice(t, db, 1, 5)
	seedDashboardService(t, db, 2, 1, "Expensive", true)
	setDashboardServicePrice(t, db, 2, 200)

	base := utcMidnight(-2)
	for i := 0; i < 3; i++ {
		seedReservationWithServices(t, db, 1, seedReservationPlan{
			date: base.AddDate(0, 0, i), startTime: "09:00", endTime: "09:30",
			status: "COMPLETED", totalPrice: 5, clientID: 1, personnelID: 1, serviceIDs: []uint{1},
		})
	}
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: base, startTime: "10:00", endTime: "11:00",
		status: "COMPLETED", totalPrice: 200, clientID: 1, personnelID: 1, serviceIDs: []uint{2},
	})

	rows, err := repo.TopServicesByRevenue(1, base, base.AddDate(0, 0, 3), 10)
	assert.NoError(t, err)
	if assert.Len(t, rows, 2) {
		assert.Equal(t, uint(2), rows[0].ServiceID, "expensive-rare outranks cheap-frequent")
		assert.Equal(t, 200.0, rows[0].Revenue)
		assert.Equal(t, int64(1), rows[0].AppointmentCount)
		assert.Equal(t, uint(1), rows[1].ServiceID)
		assert.Equal(t, 15.0, rows[1].Revenue)
		assert.Equal(t, int64(3), rows[1].AppointmentCount)
	}
}

// TestDashboardRepo_PersonnelRevenue verifies staff ranking by COMPLETED
// revenue within range.
func TestDashboardRepo_PersonnelRevenue(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)

	seedDashboardCenter(t, db, 1, "Staff Center", "staff@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardService(t, db, 1, 1, "Svc", true)

	base := utcMidnight(-1)
	// Staff 1: two COMPLETED = 210. Staff 2: one COMPLETED = 100.
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "09:00", endTime: "10:00", status: "COMPLETED", totalPrice: 150, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "11:00", endTime: "12:00", status: "COMPLETED", totalPrice: 60, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "13:00", endTime: "14:00", status: "COMPLETED", totalPrice: 100, clientID: 1, personnelID: 2, serviceIDs: []uint{1}})
	// PENDING must not count toward revenue.
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "15:00", endTime: "16:00", status: "PENDING", totalPrice: 500, clientID: 1, personnelID: 2, serviceIDs: []uint{1}})

	rows, err := repo.PersonnelRevenue(1, base, base.AddDate(0, 0, 1), 10)
	assert.NoError(t, err)
	if assert.Len(t, rows, 2) {
		assert.Equal(t, uint(1), rows[0].PersonnelID)
		assert.Equal(t, "Staff 1", rows[0].PersonnelName)
		assert.Equal(t, 210.0, rows[0].Revenue)
		assert.Equal(t, int64(2), rows[0].AppointmentCount)
		assert.Equal(t, uint(2), rows[1].PersonnelID)
		assert.Equal(t, 100.0, rows[1].Revenue, "PENDING revenue excluded")
	}
}

// TestDashboardService_HourHeatmap verifies the 7×24 matrix built in Go:
// correct day/hour placement, edge hours (00:xx, 23:xx) and status filtering.
func TestDashboardService_HourHeatmap(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)
	svc := service.NewDashboardServiceImpl(repo, nil)

	seedDashboardCenter(t, db, 1, "Matrix Center", "matrix@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Svc", true)

	base := utcMidnight(-3)
	dow := int(base.Weekday()) // index 0 = Sunday
	nextDow := (dow + 1) % 7

	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "09:15", endTime: "10:00", status: "COMPLETED", totalPrice: 100, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "09:45", endTime: "10:30", status: "COMPLETED", totalPrice: 50, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "23:59", endTime: "23:59", status: "COMPLETED", totalPrice: 20, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base.AddDate(0, 0, 1), startTime: "00:05", endTime: "01:00", status: "COMPLETED", totalPrice: 30, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	// Excluded statuses.
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "15:00", endTime: "16:00", status: "PENDING", totalPrice: 80, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "16:00", endTime: "17:00", status: "CANCELLED", totalPrice: 999, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})

	resp, err := svc.GetStatsWithRange(1, base, base.AddDate(0, 0, 1), 30)
	assert.NoError(t, err)

	require7x24(t, resp.HourHeatmap)
	assert.Equal(t, 2, resp.HourHeatmap[dow][9], "two COMPLETED start in hour 09")
	assert.Equal(t, 1, resp.HourHeatmap[dow][23], "edge hour 23 counted")
	assert.Equal(t, 1, resp.HourHeatmap[nextDow][0], "edge hour 00 lands on next day-of-week")
	assert.Equal(t, 0, resp.HourHeatmap[dow][15], "PENDING excluded")
	assert.Equal(t, 0, resp.HourHeatmap[dow][16], "CANCELLED excluded")

	total := 0
	for _, row := range resp.HourHeatmap {
		for _, v := range row {
			total += v
		}
	}
	assert.Equal(t, 4, total, "matrix sums to the number of COMPLETED reservations in range")
}

// require7x24 asserts the fixed matrix shape of the wire contract.
func require7x24(t *testing.T, m [][]int) {
	t.Helper()
	if assert.Len(t, m, 7) {
		for d, row := range m {
			assert.Len(t, row, 24, "day %d must have 24 cells", d)
		}
	}
}

// TestDashboardService_DeltaPct covers the three delta regimes through the
// public service API: growth, decline, and missing baseline.
func TestDashboardService_DeltaPct(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	newSUT := func(t *testing.T) (*gorm.DB, service.DashboardService) {
		db := setupRouterAuthzDB(t)
		repo := repository.NewDashboardRepositoryImpl(db)
		return db, service.NewDashboardServiceImpl(repo, nil)
	}
	seedBase := func(t *testing.T, db *gorm.DB) {
		seedDashboardCenter(t, db, 1, "Delta Center", "delta@test.com", true)
		seedDashboardClient(t, db, 1, "Client", "One")
		seedDashboardPersonnel(t, db, 1, 1, true)
		seedDashboardService(t, db, 1, 1, "Svc", true)
	}
	seedDay := func(t *testing.T, db *gorm.DB, day time.Time, total float64) {
		seedReservationWithServices(t, db, 1, seedReservationPlan{
			date: day, startTime: "10:00", endTime: "11:00", status: "COMPLETED",
			totalPrice: total, clientID: 1, personnelID: 1, serviceIDs: []uint{1},
		})
	}

	// Growth: today 200 vs yesterday 100 → +100.00.
	db, svc := newSUT(t)
	seedBase(t, db)
	seedDay(t, db, utcMidnight(0), 200)
	seedDay(t, db, utcMidnight(-1), 100)
	resp, err := svc.GetStatsWithRange(1, utcMidnight(0), utcMidnight(0), 30)
	assert.NoError(t, err)
	if assert.NotNil(t, resp.RevenueDeltaPct) {
		assert.Equal(t, 100.0, *resp.RevenueDeltaPct)
	}
	if assert.NotNil(t, resp.AppointmentsDeltaPct) {
		assert.Equal(t, 0.0, *resp.AppointmentsDeltaPct, "1 appointment both days → no change")
	}

	// Decline: today 50 vs yesterday 200 → -75.00.
	db, svc = newSUT(t)
	seedBase(t, db)
	seedDay(t, db, utcMidnight(0), 50)
	seedDay(t, db, utcMidnight(-1), 200)
	resp, err = svc.GetStatsWithRange(1, utcMidnight(0), utcMidnight(0), 30)
	assert.NoError(t, err)
	if assert.NotNil(t, resp.RevenueDeltaPct) {
		assert.Equal(t, -75.0, *resp.RevenueDeltaPct)
	}

	// Missing baseline: previous window empty, current has data → +100.
	db, svc = newSUT(t)
	seedBase(t, db)
	seedDay(t, db, utcMidnight(0), 80)
	resp, err = svc.GetStatsWithRange(1, utcMidnight(0), utcMidnight(0), 30)
	assert.NoError(t, err)
	if assert.NotNil(t, resp.RevenueDeltaPct) {
		assert.Equal(t, 100.0, *resp.RevenueDeltaPct)
	}
	_ = svc
}

// TestDashboardStats_Phase2_HTTP is the end-to-end contract check: deltas,
// cancellation rates, heatmap cells and leaderboards over the wire.
func TestDashboardStats_Phase2_HTTP(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := dashboardTestRouter(t)

	seedDashboardCenter(t, db, 1, "Phase2 Salon", "phase2@test.com", true)
	seedDashboardClient(t, db, 1, "Nina", "Novak")
	seedDashboardClient(t, db, 2, "Beta", "Bell")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardService(t, db, 1, 1, "Cheap", true)
	setDashboardServicePrice(t, db, 1, 5)
	seedDashboardService(t, db, 2, 1, "Expensive", true)
	setDashboardServicePrice(t, db, 2, 200)

	base := utcMidnight(-7)
	dow := int(base.Weekday())

	// Previous window (one COMPLETED, 155) → cur 310 yields exactly +100.00.
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: utcMidnight(-10), startTime: "10:00", endTime: "11:00",
		status: "COMPLETED", totalPrice: 155, clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	// Current window: 3 COMPLETED (310), 1 PENDING, 1 CANCELLED.
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "09:00", endTime: "10:00", status: "COMPLETED", totalPrice: 150, clientID: 1, personnelID: 1, serviceIDs: []uint{2}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base.AddDate(0, 0, 1), startTime: "10:00", endTime: "11:00", status: "COMPLETED", totalPrice: 100, clientID: 1, personnelID: 2, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base.AddDate(0, 0, 1), startTime: "11:00", endTime: "12:00", status: "COMPLETED", totalPrice: 60, clientID: 2, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base.AddDate(0, 0, 2), startTime: "12:00", endTime: "13:00", status: "PENDING", totalPrice: 80, clientID: 2, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base.AddDate(0, 0, 2), startTime: "13:00", endTime: "14:00", status: "CANCELLED", totalPrice: 90, clientID: 1, personnelID: 2, serviceIDs: []uint{1}})

	path := "/api/dashboard/stats?from=" + base.Format("2006-01-02") + "&to=" + utcMidnight(0).Format("2006-01-02")
	w := do(r, http.MethodGet, path, roleToken(t, 7, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusOK, w.Code)

	var got response.DashboardStatsResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))

	// Deltas: revenue 310 vs 155 → +100.00; appointments 4 vs 1 → +300.00.
	if assert.NotNil(t, got.RevenueDeltaPct) {
		assert.Equal(t, 100.0, *got.RevenueDeltaPct)
	}
	if assert.NotNil(t, got.AppointmentsDeltaPct) {
		assert.Equal(t, 300.0, *got.AppointmentsDeltaPct)
	}

	// Cancellation: 1 of 5 → 20.00%.
	assert.Equal(t, 5, got.TotalAppointments)
	assert.Equal(t, 1, got.CancelledCount)
	assert.Equal(t, 20.0, got.CancellationRatePct)

	// Heatmap cells.
	require7x24(t, got.HourHeatmap)
	w1 := (dow + 1) % 7
	assert.Equal(t, 1, got.HourHeatmap[dow][9])
	assert.Equal(t, 1, got.HourHeatmap[w1][10])
	assert.Equal(t, 1, got.HourHeatmap[w1][11])
	assert.Equal(t, 0, got.HourHeatmap[(dow+2)%7][12], "PENDING/CANCELLED never land in the matrix")

	// Personnel leaderboard: Staff 1 (210) before Staff 2 (100).
	if assert.Len(t, got.PersonnelRevenue, 2) {
		assert.Equal(t, uint(1), got.PersonnelRevenue[0].PersonnelID)
		assert.Equal(t, 210.0, got.PersonnelRevenue[0].Revenue)
		assert.Equal(t, uint(2), got.PersonnelRevenue[1].PersonnelID)
		assert.Equal(t, 100.0, got.PersonnelRevenue[1].Revenue)
	}

	// Top services flip over the wire: Expensive(200) beats Cheap(10).
	if assert.Len(t, got.TopServices, 2) {
		assert.Equal(t, uint(2), got.TopServices[0].ServiceID)
		assert.Equal(t, 200.0, got.TopServices[0].Revenue)
		assert.Equal(t, uint(1), got.TopServices[1].ServiceID)
		assert.Equal(t, 10.0, got.TopServices[1].Revenue)
	}
}

// ──────────────────────────────────────────────
//  feat-admin-analytics — Phase 3 metrics
// ──────────────────────────────────────────────

// TestDashboardRepo_ClientRecurrence verifies the NEW vs RECURRING split,
// including the lapsed-client exclusion and CANCELLED bookings never
// establishing a first visit.
func TestDashboardRepo_ClientRecurrence(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)

	seedDashboardCenter(t, db, 1, "Recurrence Center", "rec@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Svc", true)
	// Clients: 1 = recurring (visited before range + in range),
	// 2 = new (first ever visit in range), 3 = lapsed (only before range).
	seedDashboardClient(t, db, 1, "Recurring", "Rose")
	seedDashboardClient(t, db, 2, "Newcomer", "Ned")
	seedDashboardClient(t, db, 3, "Lapsed", "Lou")

	from := utcMidnight(-7)
	to := utcMidnight(0)

	// Client 1: old visit + in-range visit → RECURRING.
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: utcMidnight(-30), startTime: "09:00", endTime: "10:00", status: "COMPLETED", totalPrice: 50, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: utcMidnight(-2), startTime: "09:00", endTime: "10:00", status: "COMPLETED", totalPrice: 50, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})

	// Client 2: single first-ever visit inside the range → NEW.
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: utcMidnight(-3), startTime: "10:00", endTime: "11:00", status: "COMPLETED", totalPrice: 60, clientID: 2, personnelID: 1, serviceIDs: []uint{1}})

	// Client 3: only a pre-range visit → neither bucket.
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: utcMidnight(-30), startTime: "11:00", endTime: "12:00", status: "COMPLETED", totalPrice: 70, clientID: 3, personnelID: 1, serviceIDs: []uint{1}})

	// A pre-range CANCELLED booking must not make client 4 "existing".
	seedDashboardClient(t, db, 4, "CancelledFirst", "Cara")
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: utcMidnight(-20), startTime: "12:00", endTime: "13:00", status: "CANCELLED", totalPrice: 999, clientID: 4, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: utcMidnight(-1), startTime: "13:00", endTime: "14:00", status: "COMPLETED", totalPrice: 80, clientID: 4, personnelID: 1, serviceIDs: []uint{1}})

	row, err := repo.ClientRecurrence(1, from, to)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), row.NewClients, "clients 2 and 4 (cancelled pre-range ignored)")
	assert.Equal(t, int64(1), row.RecurringClients, "client 1 only; client 3 is lapsed")
}

// TestDashboardRepo_TopClientsBySpend proves spend ranks above frequency:
// client M spends 500 in 2 visits, client N spends 400 in 9 visits.
func TestDashboardRepo_TopClientsBySpend(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)

	seedDashboardCenter(t, db, 1, "Spend Center", "spend@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Svc", true)
	seedDashboardClient(t, db, 1, "Big", "Spender")
	seedDashboardClient(t, db, 2, "Frequent", "Fran")

	base := utcMidnight(-2)
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "09:00", endTime: "10:00", status: "COMPLETED", totalPrice: 250, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base.AddDate(0, 0, 1), startTime: "09:00", endTime: "10:00", status: "COMPLETED", totalPrice: 250, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	for i := 0; i < 9; i++ {
		seedReservationWithServices(t, db, 1, seedReservationPlan{
			date: base.AddDate(0, 0, 0), startTime: fmt.Sprintf("%02d:00", 9+i%8), endTime: "17:00",
			status: "COMPLETED", totalPrice: 45.0 + float64(i)/10, clientID: 2, personnelID: 1, serviceIDs: []uint{1},
		})
	}

	rows, err := repo.TopClientsBySpend(1, base, base.AddDate(0, 0, 2), 10)
	assert.NoError(t, err)
	if assert.Len(t, rows, 2) {
		assert.Equal(t, uint(1), rows[0].ClientID, "Big Spender leads despite fewer visits")
		assert.Equal(t, 500.0, rows[0].TotalSpent)
		assert.Equal(t, int64(2), rows[0].VisitCount)
		assert.Equal(t, uint(2), rows[1].ClientID)
	}
}

// TestDashboardRepo_AtRiskClients verifies the cutoff boundary: a last visit
// exactly at the cutoff is NOT at risk; strictly older is.
func TestDashboardRepo_AtRiskClients(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)

	seedDashboardCenter(t, db, 1, "Risk Center", "risk@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Svc", true)
	// Client 1: visited exactly 30 days ago → NOT at risk with churnDays=30.
	seedDashboardClient(t, db, 1, "Boundary", "Betty")
	// Client 2: visited exactly 31 days ago → at risk.
	seedDashboardClient(t, db, 2, "Slipping", "Sam")
	// Client 3: never completed anything → absent.
	seedDashboardClient(t, db, 3, "PendingOnly", "Pat")

	seedReservationWithServices(t, db, 1, seedReservationPlan{date: utcMidnight(-30), startTime: "09:00", endTime: "10:00", status: "COMPLETED", totalPrice: 100, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: utcMidnight(-31), startTime: "09:00", endTime: "10:00", status: "COMPLETED", totalPrice: 100, clientID: 2, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: utcMidnight(-40), startTime: "09:00", endTime: "10:00", status: "PENDING", totalPrice: 100, clientID: 3, personnelID: 1, serviceIDs: []uint{1}})

	cutoff := utcMidnight(-30)
	rows, err := repo.AtRiskClients(1, cutoff, 20)
	assert.NoError(t, err)
	if assert.Len(t, rows, 1) {
		assert.Equal(t, uint(2), rows[0].ClientID, "strictly-older-than-cutoff only")
		assert.NotEmpty(t, rows[0].LastVisit)
	}
}

// TestDashboardService_TicketAndRatingSeries verifies both Phase-3 trend
// series through the public API: zero-completion days yield ticket 0 and
// review-less days zero-fill the rating series.
func TestDashboardService_TicketAndRatingSeries(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)
	svc := service.NewDashboardServiceImpl(repo, nil)

	seedDashboardCenter(t, db, 1, "Trend Center", "trend@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Svc", true)
	seedDashboardClient(t, db, 1, "Client", "One")

	base := utcMidnight(-2)

	// Day 0: revenue 200 over 2 COMPLETED → ticket 100. One 5-star review.
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "09:00", endTime: "10:00", status: "COMPLETED", totalPrice: 120, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base, startTime: "11:00", endTime: "12:00", status: "COMPLETED", totalPrice: 80, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})
	review := &models.Review{BeautyCenterID: 1, ClientID: 1, Rating: 5, Comment: "great"}
	review.CreatedAt = base.Add(6 * time.Hour)
	if err := db.Create(review).Error; err != nil {
		t.Fatalf("failed to seed review: %v", err)
	}

	// Day 1: no COMPLETED, no reviews → zeros.

	// Day 2: one PENDING only → ticket 0 too (no completions).
	seedReservationWithServices(t, db, 1, seedReservationPlan{date: base.AddDate(0, 0, 2), startTime: "09:00", endTime: "10:00", status: "PENDING", totalPrice: 90, clientID: 1, personnelID: 1, serviceIDs: []uint{1}})

	resp, err := svc.GetStatsWithRange(1, base, base.AddDate(0, 0, 2), 30)
	assert.NoError(t, err)

	if assert.Len(t, resp.TicketSeries, 3) {
		assert.Equal(t, 100.0, resp.TicketSeries[0].Value)
		assert.Equal(t, 0.0, resp.TicketSeries[1].Value)
		assert.Equal(t, 0.0, resp.TicketSeries[2].Value, "PENDING-only day has no completions")
	}
	if assert.Len(t, resp.RatingSeries, 3) {
		assert.Equal(t, 5.0, resp.RatingSeries[0].Value)
		assert.Equal(t, 0.0, resp.RatingSeries[1].Value, "reviewless day zero-fills")
		assert.Equal(t, 0.0, resp.RatingSeries[2].Value)
	}

	// New vs recurring flows through the same payload.
	assert.Equal(t, 1, resp.NewClients)
	assert.Equal(t, 0, resp.RecurringClients)
}

// TestDashboardService_Occupancy uses center-level working hours as the
// shared denominator (D7 revision): Mon 09:00-17:00 = 480 min per Monday.
func TestDashboardService_Occupancy(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)
	svc := service.NewDashboardServiceImpl(repo, nil)

	seedDashboardCenter(t, db, 1, "Occupancy Center", "occ@test.com", true)
	seedDashboardClient(t, db, 1, "Client", "One")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardService(t, db, 1, 1, "Svc", true)

	// Find next Monday so the schedule applies to exactly one range day.
	now := time.Now().UTC()
	offset := (8 - int(now.Weekday())) % 7 // days until next Monday
	if offset == 0 {
		offset = 7
	}
	monday := utcMidnight(offset)

	wh := &models.WorkingHour{BeautyCenterID: 1, DayOfWeek: 1, StartTime: "09:00", EndTime: "17:00"}
	if err := db.Create(wh).Error; err != nil {
		t.Fatalf("failed to seed working hour: %v", err)
	}

	// Staff 1 books 09:00-13:00 (240 of 480 → 50%). Staff 2 has no bookings.
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: monday, startTime: "09:00", endTime: "13:00", status: "CONFIRMED",
		totalPrice: 100, clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	resp, err := svc.GetStatsWithRange(1, monday, monday, 30)
	assert.NoError(t, err)

	if assert.Len(t, resp.PersonnelOccupancy, 1, "staff without bookings are omitted") {
		got := resp.PersonnelOccupancy[0]
		assert.Equal(t, uint(1), got.PersonnelID)
		assert.Equal(t, "Staff 1", got.PersonnelName)
		assert.Equal(t, 240, got.BookedMinutes)
		assert.Equal(t, 480, got.AvailableMinutes)
		assert.Equal(t, 50.0, got.OccupancyPct)
	}
}

// TestDashboardStats_Phase3_HTTP covers the wire contract end-to-end:
// churnDays validation, recurrence counters and leaderboard presence.
func TestDashboardStats_Phase3_HTTP(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := dashboardTestRouter(t)

	seedDashboardCenter(t, db, 1, "Phase3 Salon", "phase3@test.com", true)
	seedDashboardClient(t, db, 1, "Nina", "Novak")
	seedDashboardClient(t, db, 2, "Beta", "Bell")
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Svc", true)

	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: utcMidnight(-2), startTime: "10:00", endTime: "11:00", status: "COMPLETED",
		totalPrice: 300, clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	t.Run("churnDays out of bounds is 400", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/dashboard/stats?churnDays=0", roleToken(t, 7, 1, "beauty_center"), "")
		assert.Equal(t, http.StatusBadRequest, w.Code)
		w = do(r, http.MethodGet, "/api/dashboard/stats?churnDays=999", roleToken(t, 7, 1, "beauty_center"), "")
		assert.Equal(t, http.StatusBadRequest, w.Code)
		w = do(r, http.MethodGet, "/api/dashboard/stats?churnDays=abc", roleToken(t, 7, 1, "beauty_center"), "")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("valid churnDays returns full payload", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/dashboard/stats?churnDays=60", roleToken(t, 7, 1, "beauty_center"), "")
		assert.Equal(t, http.StatusOK, w.Code)

		var got response.DashboardStatsResponse
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))

		// Client 1's visit 2 days ago is her first ever → NEW, not recurring.
		assert.Equal(t, 1, got.NewClients)
		assert.Equal(t, 0, got.RecurringClients)

		// Top clients: Nina with 300.
		if assert.Len(t, got.TopClients, 1) {
			assert.Equal(t, uint(1), got.TopClients[0].ClientID)
			assert.Equal(t, "Nina Novak", got.TopClients[0].ClientName)
			assert.Equal(t, 300.0, got.TopClients[0].TotalSpent)
			assert.Equal(t, 1, got.TopClients[0].VisitCount)
		}

		// At-risk: churnDays=60 → nothing older than 60d → empty but non-null.
		assert.Empty(t, got.AtRiskClients)
		assert.Contains(t, w.Body.String(), `"at_risk_clients":[]`)

		// Trend series present with default 30-day window; the visit 2 days
		// ago sits at index 27 (today-29 … today).
		if assert.Len(t, got.TicketSeries, 30) {
			assert.Equal(t, 300.0, got.TicketSeries[27].Value)
			assert.Equal(t, 0.0, got.TicketSeries[0].Value)
		}
		if assert.Len(t, got.RatingSeries, 30) {
			assert.Equal(t, 0.0, got.RatingSeries[27].Value, "no reviews seeded")
		}

		// Occupancy empty (no working hours seeded) — never null.
		assert.Empty(t, got.PersonnelOccupancy)
		assert.Contains(t, w.Body.String(), `"personnel_occupancy":[]`)
	})
}
