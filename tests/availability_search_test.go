package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/router"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  Cross-center availability search
// ──────────────────────────────────────────────

// availabilityTestRouter mounts the search router AND the per-center
// beauty-center router over the same SQLite DB, so the consistency test can
// compare the two paths directly.
func availabilityTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	validate := validator.New()

	r := gin.New()
	api := r.Group("/api")
	router.AvailabilityRoutes(api, db, validate)
	router.BeautyCenterRoutes(api, db, validate)
	return r, db
}

func seedWorkingHour(t *testing.T, db *gorm.DB, centerID uint, dayOfWeek int, start, end string) {
	t.Helper()
	require.NoError(t, db.Create(&models.WorkingHour{
		BeautyCenterID: centerID,
		DayOfWeek:      dayOfWeek,
		StartTime:      start,
		EndTime:        end,
	}).Error)
}

func seedServiceRow(t *testing.T, db *gorm.DB, id, centerID uint, duration int) {
	t.Helper()
	require.NoError(t, db.Create(&models.Service{
		ID:             id,
		BeautyCenterID: centerID,
		CategoryID:     1,
		Name:           fmt.Sprintf("svc-%d", id),
		Description:    "desc",
		Duration:       duration,
		Price:          50,
		IsActive:       true,
	}).Error)
}

func seedReservationRow(t *testing.T, db *gorm.DB, centerID, personnelID uint, date time.Time, start, end string) {
	t.Helper()
	require.NoError(t, db.Create(&models.Reservation{
		BeautyCenterID: centerID,
		ClientID:       1,
		PersonnelID:    personnelID,
		Date:           date,
		StartTime:      start,
		EndTime:        end,
		TotalPrice:     50,
		Status:         "CONFIRMED",
	}).Error)
}

type searchEnvelope struct {
	Results []response.AvailabilitySlotResult `json:"results"`
	Total   int                               `json:"total"`
}

func getSearch(t *testing.T, r *gin.Engine, query string) searchEnvelope {
	t.Helper()
	w := do(r, http.MethodGet, "/api/availability/search?"+query, "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var env searchEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	return env
}

func getPerCenterAvailable(t *testing.T, r *gin.Engine, path string) []string {
	t.Helper()
	w := do(r, http.MethodGet, path, "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body response.AvailableSlotsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	times := make([]string, 0)
	for _, s := range body.Slots {
		if s.Available {
			times = append(times, s.Time)
		}
	}
	return times
}

func searchStartTimes(env searchEnvelope) []string {
	times := make([]string, 0, len(env.Results))
	for _, r := range env.Results {
		times = append(times, r.StartTime)
	}
	return times
}

// TestAvailabilitySearch_ConsistentWithPerCenter is THE acceptance guard: for a
// given (center, date, duration, personnel) the slots returned by the search
// must be EXACTLY the slots returned by the existing per-center endpoint. Both
// paths run the same slot kernel; if they can ever disagree the feature is
// worse than useless.
func TestAvailabilitySearch_ConsistentWithPerCenter(t *testing.T) {
	r, db := availabilityTestRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	// Two services → duration 45 (15 + 30), the booking-flow aggregation.
	seedServiceRow(t, db, 10, 1, 15)
	seedServiceRow(t, db, 11, 1, 30)

	date := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC) // Monday
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")
	// One booking for personnel 5 and one for personnel 7.
	seedReservationRow(t, db, 1, 5, date, "10:00", "11:00")
	seedReservationRow(t, db, 1, 7, date, "14:00", "15:00")

	cases := []struct {
		name        string
		personnelID uint
	}{
		{"personnel scoped", 5},
		{"whole center", 0},
		{"other personnel", 9},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			query := fmt.Sprintf("service_ids=10,11&from=2026-10-05&to=2026-10-05&center_id=1&limit=50&personnel_id=%d", tc.personnelID)
			env := getSearch(t, r, query)

			perCenter := getPerCenterAvailable(t, r,
				fmt.Sprintf("/api/beauty-centers/1/available-slots?date=2026-10-05&duration=45&personnel_id=%d", tc.personnelID))

			search := searchStartTimes(env)
			assert.Equal(t, perCenter, search,
				"search slots must equal per-center slots for (center=1, date=2026-10-05, duration=45, personnel=%d)", tc.personnelID)
			assert.Equal(t, len(perCenter), env.Total)
		})
	}
}

// TestAvailabilitySearch_EchoesPersonnelOnlyWhenSupplied guards the optional
// personnel_id echo: present when supplied, omitted (nil) otherwise.
func TestAvailabilitySearch_EchoesPersonnelOnlyWhenSupplied(t *testing.T) {
	r, db := availabilityTestRouter(t)
	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedServiceRow(t, db, 10, 1, 30)
	date := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")

	withPersonnel := getSearch(t, r, "service_ids=10&from=2026-10-05&to=2026-10-05&personnel_id=5")
	require.NotEmpty(t, withPersonnel.Results)
	require.NotNil(t, withPersonnel.Results[0].PersonnelID)
	assert.Equal(t, uint(5), *withPersonnel.Results[0].PersonnelID)

	withoutPersonnel := getSearch(t, r, "service_ids=10&from=2026-10-05&to=2026-10-05")
	require.NotEmpty(t, withoutPersonnel.Results)
	assert.Nil(t, withoutPersonnel.Results[0].PersonnelID)
}

// TestAvailabilitySearch_Validation covers the 400 contract for bad input.
func TestAvailabilitySearch_Validation(t *testing.T) {
	r, db := availabilityTestRouter(t)
	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedServiceRow(t, db, 10, 1, 30)

	cases := []struct {
		name  string
		query string
		want  int
	}{
		{"missing service_ids", "from=2026-10-01&to=2026-10-02", http.StatusBadRequest},
		{"malformed service_ids", "service_ids=abc&from=2026-10-01&to=2026-10-02", http.StatusBadRequest},
		{"unknown service", "service_ids=999&from=2026-10-01&to=2026-10-02", http.StatusBadRequest},
		{"missing from", "service_ids=10&to=2026-10-02", http.StatusBadRequest},
		{"missing to", "service_ids=10&from=2026-10-01", http.StatusBadRequest},
		{"bad from date", "service_ids=10&from=2026-13-01&to=2026-10-02", http.StatusBadRequest},
		{"bad to date", "service_ids=10&from=2026-10-01&to=nope", http.StatusBadRequest},
		{"reversed range", "service_ids=10&from=2026-10-05&to=2026-10-01", http.StatusBadRequest},
		{"range over 14 days", "service_ids=10&from=2026-10-01&to=2026-10-16", http.StatusBadRequest},
		{"bad period", "service_ids=10&from=2026-10-01&to=2026-10-02&period=dawn", http.StatusBadRequest},
		{"range exactly 14 days", "service_ids=10&from=2026-10-01&to=2026-10-15", http.StatusOK},
		{"bad center_id", "service_ids=10&from=2026-10-01&to=2026-10-02&center_id=-1", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := do(r, http.MethodGet, "/api/availability/search?"+tc.query, "", "")
			assert.Equal(t, tc.want, w.Code, w.Body.String())
		})
	}
}

// TestAvailabilitySearch_PeriodFilter checks the documented boundaries:
// morning [00:00,12:00), afternoon [12:00,18:00), evening [18:00,24:00).
func TestAvailabilitySearch_PeriodFilter(t *testing.T) {
	r, db := availabilityTestRouter(t)
	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedServiceRow(t, db, 10, 1, 30)
	date := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	seedWorkingHour(t, db, 1, int(date.Weekday()), "08:00", "22:00")

	morning := getSearch(t, r, "service_ids=10&from=2026-10-05&to=2026-10-05&period=morning")
	for _, res := range morning.Results {
		assert.Less(t, res.StartTime, "12:00")
	}
	afternoon := getSearch(t, r, "service_ids=10&from=2026-10-05&to=2026-10-05&period=afternoon")
	require.NotEmpty(t, afternoon.Results)
	for _, res := range afternoon.Results {
		assert.GreaterOrEqual(t, res.StartTime, "12:00")
		assert.Less(t, res.StartTime, "18:00")
	}
	evening := getSearch(t, r, "service_ids=10&from=2026-10-05&to=2026-10-05&period=evening")
	require.NotEmpty(t, evening.Results)
	for _, res := range evening.Results {
		assert.GreaterOrEqual(t, res.StartTime, "18:00")
	}

	// Every unbounded slot must be partitioned across the three periods.
	all := getSearch(t, r, "service_ids=10&from=2026-10-05&to=2026-10-05&limit=50")
	assert.Equal(t, all.Total, morning.Total+afternoon.Total+evening.Total)
}

// TestAvailabilitySearch_LimitClamp verifies the default (20) and max (50) and
// that Total reports the full match count before truncation.
func TestAvailabilitySearch_LimitClamp(t *testing.T) {
	r, db := availabilityTestRouter(t)
	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedServiceRow(t, db, 10, 1, 30)
	// 7 days x a long block → far more than 50 slots.
	seedWorkingHour(t, db, 1, 0, "08:00", "20:00")
	seedWorkingHour(t, db, 1, 1, "08:00", "20:00")
	seedWorkingHour(t, db, 1, 2, "08:00", "20:00")
	seedWorkingHour(t, db, 1, 3, "08:00", "20:00")
	seedWorkingHour(t, db, 1, 4, "08:00", "20:00")
	seedWorkingHour(t, db, 1, 5, "08:00", "20:00")
	seedWorkingHour(t, db, 1, 6, "08:00", "20:00")

	defaultLimit := getSearch(t, r, "service_ids=10&from=2026-10-05&to=2026-10-11")
	assert.Len(t, defaultLimit.Results, 20)
	assert.Greater(t, defaultLimit.Total, 20)

	maxLimit := getSearch(t, r, "service_ids=10&from=2026-10-05&to=2026-10-11&limit=999")
	assert.Len(t, maxLimit.Results, 50)
	assert.Equal(t, defaultLimit.Total, maxLimit.Total, "Total is independent of limit")

	small := getSearch(t, r, "service_ids=10&from=2026-10-05&to=2026-10-11&limit=5")
	assert.Len(t, small.Results, 5)
}

// ──────────────────────────────────────────────
//  Query budget — no per-center / per-day storm
// ──────────────────────────────────────────────

// measureSearchQueries seeds 5 active centers sharing a 7-day work week and
// returns the SELECT count for one search request spanning those 7 days.
func measureSearchQueries(t *testing.T, r *gin.Engine, counter *int64) int64 {
	t.Helper()
	atomic.StoreInt64(counter, 0)
	w := do(r, http.MethodGet,
		"/api/availability/search?service_ids=10&from=2026-10-05&to=2026-10-11&limit=50", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return atomic.LoadInt64(counter)
}

// TestAvailabilitySearch_QueryCountIsBounded proves the search loads candidate
// centers, work hours and reservations in a BOUNDED number of SELECTs, not one
// per center or per day. Before the batched reads a 5-center x 7-day search
// would fan out to two queries per center-day (70 SELECTs); after it stays a
// small constant. It also shows the looped per-center endpoint issuing the
// storm the batched reads replace.
func TestAvailabilitySearch_QueryCountIsBounded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	counter := attachQueryCounter(db)

	r := gin.New()
	api := r.Group("/api")
	router.AvailabilityRoutes(api, db, validator.New())
	router.BeautyCenterRoutes(api, db, validator.New())

	const centers, days = 5, 7
	seedServiceRow(t, db, 10, 1, 30)
	for c := uint(1); c <= centers; c++ {
		seedDashboardCenter(t, db, c, fmt.Sprintf("Center %d", c), fmt.Sprintf("c%d@test.com", c), true)
		for d := 0; d < 7; d++ {
			seedWorkingHour(t, db, c, d, "08:00", "20:00")
		}
		seedReservationRow(t, db, c, 1, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), "10:00", "11:00")
	}

	searchQueries := measureSearchQueries(t, r, counter)
	t.Logf("availability search SELECTs — %d centers x %d days: %d", centers, days, searchQueries)

	// The naive shape: per-center endpoint called once per center per day.
	atomic.StoreInt64(counter, 0)
	for c := uint(1); c <= centers; c++ {
		for d := 0; d < days; d++ {
			date := time.Date(2026, 10, 5+d, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
			w := do(r, http.MethodGet,
				fmt.Sprintf("/api/beauty-centers/%d/available-slots?date=%s&duration=30", c, date), "", "")
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		}
	}
	perCenterQueries := atomic.LoadInt64(counter)
	t.Logf("per-center loop SELECTs — %d centers x %d days: %d", centers, days, perCenterQueries)

	assert.Lessf(t, searchQueries, int64(10),
		"search must use a bounded number of SELECTs, got %d", searchQueries)
	assert.Greaterf(t, perCenterQueries, searchQueries*5,
		"batched search must be far cheaper than looping per-center (%d vs %d)", searchQueries, perCenterQueries)
}
