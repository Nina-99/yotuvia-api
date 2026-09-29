package tests

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/Nina-99/yottu-api/router"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  PART B — GET /api/reservation/me/metrics
// ──────────────────────────────────────────────

// metricsBody mirrors the frozen wire shape the endpoint must return.
type metricsBody struct {
	Range struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"range"`
	Totals struct {
		Appointments int     `json:"appointments"`
		Completed    int     `json:"completed"`
		Cancelled    int     `json:"cancelled"`
		NoShow       int     `json:"no_show"`
		Revenue      float64 `json:"revenue"`
		Currency     string  `json:"currency"`
	} `json:"totals"`
	RevenueSeries []struct {
		Date  string  `json:"date"`
		Value float64 `json:"value"`
	} `json:"revenue_series"`
	AppointmentsSeries []struct {
		Date  string  `json:"date"`
		Value float64 `json:"value"`
	} `json:"appointments_series"`
	TopServices []struct {
		ServiceID   uint    `json:"service_id"`
		ServiceName string  `json:"service_name"`
		Revenue     float64 `json:"revenue"`
		Count       int     `json:"count"`
	} `json:"top_services"`
}

func seriesValue(series []struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}, date string) (float64, bool) {
	for _, s := range series {
		if s.Date == date {
			return s.Value, true
		}
	}
	return 0, false
}

// getMetrics issues the endpoint and decodes the body.
func getMetrics(t *testing.T, r *gin.Engine, token, query string) metricsBody {
	t.Helper()
	w := do(r, http.MethodGet, "/api/reservation/me/metrics"+query, token, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body metricsBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

// TestPersonnelMetrics_ShapePerLineRevenueAndRange is the acceptance test for
// the frozen shape and every semantic:
//   - revenue counts COMPLETED only and attributes per LINE (the non-lead pro 2
//     earns their 100, not the lead's 300);
//   - a booking OUTSIDE the requested range is excluded entirely;
//   - a CONFIRMED booking counts as an appointment but adds no revenue;
//   - both series cover the whole range with ZERO-filled gaps;
//   - currency comes from the centre.
func TestPersonnelMetrics_ShapePerLineRevenueAndRange(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Metrics Center", "metrics@test.com", true) // currency default USD
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Metrics", "Client")

	// In-range COMPLETED shared booking: lead 1, line 2 → pro 2. Total 300.
	seedReservationWithPricedLines(t, db, 1, 1, 1, mpDay, "10:00", "11:00", "COMPLETED", 300, []reservedLine{
		{serviceID: 1, personnelID: 1, price: 200, duration: 30},
		{serviceID: 2, personnelID: 2, price: 100, duration: 30},
	})

	// In-range CONFIRMED booking for pro 2: an appointment, never revenue.
	seedReservationWithPricedLines(t, db, 1, 1, 2, mpDay, "12:00", "12:30", "CONFIRMED", 50, []reservedLine{
		{serviceID: 1, personnelID: 2, price: 50, duration: 30},
	})

	// OUT-OF-range COMPLETED booking for pro 2: must be excluded everywhere.
	outside := mpDay.AddDate(0, 0, 46) // 2026-11-20
	seedReservationWithPricedLines(t, db, 1, 1, 2, outside, "10:00", "10:30", "COMPLETED", 999, []reservedLine{
		{serviceID: 1, personnelID: 2, price: 999, duration: 30},
	})

	body := getMetrics(t, r, roleToken(t, 2, 1, "personnel"), "?from=2026-10-01&to=2026-10-31")

	// range
	assert.Equal(t, "2026-10-01", body.Range.From)
	assert.Equal(t, "2026-10-31", body.Range.To)

	// totals: 2 non-cancelled in range, 1 completed, per-LINE revenue 100 (not 300).
	assert.Equal(t, 2, body.Totals.Appointments)
	assert.Equal(t, 1, body.Totals.Completed)
	assert.Equal(t, 0, body.Totals.Cancelled)
	assert.Equal(t, 0, body.Totals.NoShow)
	assert.Equal(t, 100.0, body.Totals.Revenue, "pro 2 earns only their own line")
	assert.Equal(t, "USD", body.Totals.Currency)

	// zero-filled series across the 31-day range
	require.Len(t, body.RevenueSeries, 31)
	require.Len(t, body.AppointmentsSeries, 31)
	assert.Equal(t, "2026-10-01", body.RevenueSeries[0].Date)
	assert.Equal(t, "2026-10-31", body.RevenueSeries[30].Date)

	v, ok := seriesValue(body.RevenueSeries, "2026-10-05")
	require.True(t, ok)
	assert.Equal(t, 100.0, v)
	zero, ok := seriesValue(body.RevenueSeries, "2026-10-06")
	require.True(t, ok)
	assert.Equal(t, 0.0, zero, "gaps are zero-filled, never omitted")

	a, ok := seriesValue(body.AppointmentsSeries, "2026-10-05")
	require.True(t, ok)
	assert.Equal(t, 2.0, a, "both non-cancelled bookings count as appointments")
	az, ok := seriesValue(body.AppointmentsSeries, "2026-10-07")
	require.True(t, ok)
	assert.Equal(t, 0.0, az)

	// top services: only the COMPLETED line, scoped to pro 2.
	require.Len(t, body.TopServices, 1)
	assert.Equal(t, uint(2), body.TopServices[0].ServiceID)
	assert.Equal(t, 100.0, body.TopServices[0].Revenue)
	assert.Equal(t, 1, body.TopServices[0].Count)

	// Per-line proof on the other side: the lead reports 200, not 300.
	lead := getMetrics(t, r, roleToken(t, 1, 1, "personnel"), "?from=2026-10-01&to=2026-10-31")
	assert.Equal(t, 200.0, lead.Totals.Revenue, "the lead earns only their own line")
}

// TestPersonnelMetrics_MalformedRangeIsBadRequest proves malformed dates are 400.
func TestPersonnelMetrics_MalformedRangeIsBadRequest(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, _ := multiProfessionalRouter(t)

	w := do(r, http.MethodGet, "/api/reservation/me/metrics?from=not-a-date", roleToken(t, 2, 1, "personnel"), "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = do(r, http.MethodGet, "/api/reservation/me/metrics?to=2026-13-40", roleToken(t, 2, 1, "personnel"), "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// attachStatementCounter counts every SELECT a metrics request runs. It hooks
// BOTH the query and the row callbacks: GORM's aggregate Scan() calls run
// through the Row callback, while Find/Count/Pluck run through Query. Hooking
// only one silently under-counts (the older attachQueryCounter helper predates
// Scan usage and is left untouched). Counts Query + Row = all SELECTs.
func attachStatementCounter(db *gorm.DB) *int64 {
	var count int64
	db.Callback().Query().After("gorm:query").Register("tests:metrics_count_query", func(*gorm.DB) {
		atomic.AddInt64(&count, 1)
	})
	db.Callback().Row().After("gorm:row").Register("tests:metrics_count_row", func(*gorm.DB) {
		atomic.AddInt64(&count, 1)
	})
	return &count
}

// metricsSelectCount seeds `n` COMPLETED bookings for pro 2 and returns the
// number of SELECTs one metrics request runs.
func metricsSelectCount(t *testing.T, n int) int64 {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	counter := attachStatementCounter(db)

	r := gin.New()
	api := r.Group("/api")
	router.ReservationRoutes(api, db, validator.New())

	seedDashboardCenter(t, db, 1, "Budget Center", "budget@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Budget", "Client")

	for i := 0; i < n; i++ {
		day := mpDay.AddDate(0, 0, i)
		seedReservationWithPricedLines(t, db, 1, 1, 2, day, "10:00", "10:30", "COMPLETED", 50, []reservedLine{
			{serviceID: 1, personnelID: 2, price: 50, duration: 30},
		})
	}

	atomic.StoreInt64(counter, 0)
	w := do(r, http.MethodGet, "/api/reservation/me/metrics?from=2026-10-01&to=2026-11-30",
		roleToken(t, 2, 1, "personnel"), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return atomic.LoadInt64(counter)
}

// TestPersonnelMetrics_QueryCountIsBounded proves the endpoint is a constant
// number of aggregate queries and does NOT grow with the number of bookings.
func TestPersonnelMetrics_QueryCountIsBounded(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	small := metricsSelectCount(t, 1)
	large := metricsSelectCount(t, 30)

	t.Logf("personnel metrics SELECTs — 1 booking: %d, 30 bookings: %d", small, large)

	assert.LessOrEqualf(t, large, int64(6), "bounded budget, got %d SELECTs", large)
	assert.LessOrEqualf(t, large-small, int64(1),
		"query count must not scale with bookings: 1=%d, 30=%d", small, large)
}
