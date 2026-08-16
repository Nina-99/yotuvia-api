package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
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

	// Top services: S2=5 (today2,yesterday,u1,u4,+completed_past), S1=4, S3=2
	assert.Len(t, got.TopServices, 3)
	if assert.Len(t, got.TopServices, 3) {
		assert.Equal(t, uint(2), got.TopServices[0].ServiceID)
		assert.Equal(t, "Color", got.TopServices[0].ServiceName)
		assert.Equal(t, 5, got.TopServices[0].AppointmentCount)
		assert.Equal(t, uint(1), got.TopServices[1].ServiceID)
		assert.Equal(t, "Corte", got.TopServices[1].ServiceName)
		assert.Equal(t, 4, got.TopServices[1].AppointmentCount)
		assert.Equal(t, uint(3), got.TopServices[2].ServiceID)
		assert.Equal(t, "Peinado", got.TopServices[2].ServiceName)
		assert.Equal(t, 2, got.TopServices[2].AppointmentCount)
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

	// Wire contract: arrays must serialize as [] — never null.
	assert.Contains(t, w.Body.String(), `"top_services":[]`)
	assert.Contains(t, w.Body.String(), `"upcoming_reservations":[]`)
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

	// 5 past CONFIRMED reservations with cumulative service counts:
	// S1=5, S2=4, S3=3, S4=2, S5=1 → top 3 must be S1, S2, S3.
	for i := 1; i <= 5; i++ {
		var sids []uint
		for j := 1; j <= i; j++ {
			sids = append(sids, uint(j))
		}
		seedReservationWithServices(t, db, 1, seedReservationPlan{
			date: utcMidnight(-i), startTime: "09:00", endTime: "10:00",
			status: "CONFIRMED", totalPrice: 60, clientID: uint(i), personnelID: 1, serviceIDs: sids,
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