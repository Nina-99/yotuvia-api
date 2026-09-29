package tests

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/router"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ──────────────────────────────────────────────
//  GET /api/reservation/me/metrics — previous window + occupancy
// ──────────────────────────────────────────────

type metricsRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type metricsTotals struct {
	Appointments int     `json:"appointments"`
	Completed    int     `json:"completed"`
	Cancelled    int     `json:"cancelled"`
	NoShow       int     `json:"no_show"`
	Revenue      float64 `json:"revenue"`
	Currency     string  `json:"currency"`
}

// metricsExtBody mirrors the additive blocks without touching the shared
// metricsBody in reservation_metrics_test.go.
type metricsExtBody struct {
	Range    metricsRange  `json:"range"`
	Totals   metricsTotals `json:"totals"`
	Previous struct {
		Range  metricsRange  `json:"range"`
		Totals metricsTotals `json:"totals"`
	} `json:"previous"`
	Occupancy struct {
		AvailableMinutes int     `json:"available_minutes"`
		BookedMinutes    int     `json:"booked_minutes"`
		RatePct          float64 `json:"rate_pct"`
		Basis            string  `json:"basis"`
	} `json:"occupancy"`
}

func getMetricsExt(t *testing.T, r *gin.Engine, token, query string) metricsExtBody {
	t.Helper()
	w := do(r, http.MethodGet, "/api/reservation/me/metrics"+query, token, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body metricsExtBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "occupancy/previous must be JSON-safe (no NaN/Inf)")
	return body
}

// TestPersonnelMetrics_PreviousWindowIsSameLengthAndImmediatelyPrecedes proves
// the exact previous-range arithmetic for an October request: n = 31 days, so
// previous = [Aug 31, Sep 30]. Boundary days are seeded explicitly:
//
//   - Aug 30 (the day BEFORE the previous window) is excluded;
//   - Aug 31 (the previous window's FIRST day) counts;
//   - Sep 30 (the previous window's LAST day) counts;
//   - Oct 1  (the requested window's first day) counts as current;
//   - Nov 1  (the day after the requested window) is excluded.
func TestPersonnelMetrics_PreviousWindowIsSameLengthAndImmediatelyPrecedes(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Prev Center", "prev@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Prev", "Client")

	day := func(y int, m time.Month, d int) time.Time {
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	seed := func(date time.Time, price float64) {
		seedReservationWithPricedLines(t, db, 1, 1, 2, date, "10:00", "10:30", "COMPLETED", price, []reservedLine{
			{serviceID: 1, personnelID: 2, price: price, duration: 30},
		})
	}
	seed(day(2026, time.August, 30), 999)    // before previous → excluded
	seed(day(2026, time.August, 31), 300)    // previous first day → counted
	seed(day(2026, time.September, 30), 100) // previous last day → counted
	seed(day(2026, time.October, 1), 200)    // current first day → counted current
	seed(day(2026, time.November, 1), 777)   // after current → excluded

	body := getMetricsExt(t, r, roleToken(t, 2, 1, "personnel"), "?from=2026-10-01&to=2026-10-31")

	// Requested range echoed unchanged.
	assert.Equal(t, "2026-10-01", body.Range.From)
	assert.Equal(t, "2026-10-31", body.Range.To)

	// Current window sees exactly Oct 1's booking.
	assert.Equal(t, 1, body.Totals.Appointments)
	assert.Equal(t, 1, body.Totals.Completed)
	assert.Equal(t, 200.0, body.Totals.Revenue)

	// Previous window is the 31 days immediately before: [Aug 31, Sep 30].
	assert.Equal(t, "2026-08-31", body.Previous.Range.From, "previous starts n days before `from`")
	assert.Equal(t, "2026-09-30", body.Previous.Range.To, "previous ends the day before `from`")
	assert.Equal(t, 2, body.Previous.Totals.Appointments, "Aug 30 and Nov 1 are outside every window")
	assert.Equal(t, 2, body.Previous.Totals.Completed)
	assert.Equal(t, 400.0, body.Previous.Totals.Revenue, "300 (Aug 31) + 100 (Sep 30)")
	assert.Equal(t, body.Totals.Currency, body.Previous.Totals.Currency)
	assert.Equal(t, "USD", body.Previous.Totals.Currency)
}

// TestPersonnelMetrics_PreviousWindowAcrossMonthLengths checks a 30-day request
// (September) yields a 30-day previous window (Aug 2 → Aug 31), so the
// arithmetic is length-preserving rather than calendar-month based.
func TestPersonnelMetrics_PreviousWindowAcrossMonthLengths(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, _ := multiProfessionalRouter(t)

	body := getMetricsExt(t, r, roleToken(t, 2, 1, "personnel"), "?from=2026-09-01&to=2026-09-30")
	assert.Equal(t, "2026-08-02", body.Previous.Range.From, "30-day window → 30-day previous")
	assert.Equal(t, "2026-08-31", body.Previous.Range.To)
}

// TestPersonnelMetrics_OccupancyCountsOnlyOwnSegments proves occupancy reuses
// the shared segment kernel: a 10:00–11:00 booking split 30/30 across pro 1
// (lead) and pro 2 credits EACH 30 booked minutes — the non-lead never inherits
// the lead's whole envelope. Available minutes are the centre's Monday block.
func TestPersonnelMetrics_OccupancyCountsOnlyOwnSegments(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Occ Center", "occ@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Occ", "Client")
	// mpDay is Monday → 08:00–12:00 = 240 available minutes.
	seedWorkingHour(t, db, 1, int(mpDay.Weekday()), "08:00", "12:00")

	seedReservationWithPricedLines(t, db, 1, 1, 1, mpDay, "10:00", "11:00", "CONFIRMED", 100, []reservedLine{
		{serviceID: 1, personnelID: 1, price: 50, duration: 30},
		{serviceID: 2, personnelID: 2, price: 50, duration: 30},
	})

	query := "?from=2026-10-05&to=2026-10-05"

	pro2 := getMetricsExt(t, r, roleToken(t, 2, 1, "personnel"), query)
	assert.Equal(t, 30, pro2.Occupancy.BookedMinutes, "pro 2 counts only their own 30-minute segment")
	assert.Equal(t, 240, pro2.Occupancy.AvailableMinutes)
	assert.Equal(t, 12.5, pro2.Occupancy.RatePct, "30/240 × 100")
	assert.Equal(t, "center_hours", pro2.Occupancy.Basis)

	pro1 := getMetricsExt(t, r, roleToken(t, 1, 1, "personnel"), query)
	assert.Equal(t, 30, pro1.Occupancy.BookedMinutes, "the lead counts only their own segment too")
}

// TestPersonnelMetrics_OccupancyAvailableFollowsWeekdayHours proves the
// denominator is the sum of the centre's working blocks for the actual
// weekdays in range: Mon 08:00–12:00 (240) + Tue 09:00–18:00 (540) = 780.
func TestPersonnelMetrics_OccupancyAvailableFollowsWeekdayHours(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Hours Center", "hours@test.com", true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Hours", "Client")
	seedWorkingHour(t, db, 1, int(time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC).Weekday()), "08:00", "12:00")
	seedWorkingHour(t, db, 1, int(time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC).Weekday()), "09:00", "18:00")

	body := getMetricsExt(t, r, roleToken(t, 2, 1, "personnel"), "?from=2026-10-05&to=2026-10-06")
	assert.Equal(t, 780, body.Occupancy.AvailableMinutes, "240 (Mon) + 540 (Tue)")
	assert.Equal(t, 0, body.Occupancy.BookedMinutes)
	assert.Equal(t, 0.0, body.Occupancy.RatePct)
	assert.Equal(t, "center_hours", body.Occupancy.Basis)
}

// TestPersonnelMetrics_OccupancyZeroHoursIsSane proves the zero-denominator
// guard: no working hours in range → available 0 and rate 0 (never NaN/Inf),
// while the raw booked minutes stay truthful. json.Unmarshal in getMetricsExt
// would fail outright if the handler emitted NaN.
func TestPersonnelMetrics_OccupancyZeroHoursIsSane(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "NoHours Center", "nohours@test.com", true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "NoHours", "Client")
	// Deliberately NO working hours seeded.
	seedReservationWithPricedLines(t, db, 1, 1, 2, mpDay, "10:00", "10:30", "COMPLETED", 50, []reservedLine{
		{serviceID: 1, personnelID: 2, price: 50, duration: 30},
	})

	body := getMetricsExt(t, r, roleToken(t, 2, 1, "personnel"), "?from=2026-10-05&to=2026-10-05")
	assert.Equal(t, 0, body.Occupancy.AvailableMinutes)
	assert.Equal(t, 30, body.Occupancy.BookedMinutes, "the booking is still reported")
	assert.Equal(t, 0.0, body.Occupancy.RatePct, "no denominator → 0, never NaN/Inf")
	assert.Equal(t, "center_hours", body.Occupancy.Basis)
}

// metricsSelectCountRange returns the SELECT count for a metrics request over
// `days` consecutive days, with one full working block per weekday.
func metricsSelectCountRange(t *testing.T, days int) int64 {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	counter := attachStatementCounter(db)

	r := gin.New()
	api := r.Group("/api")
	router.ReservationRoutes(api, db, validator.New())

	seedDashboardCenter(t, db, 1, "Range Center", "range@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Range", "Client")
	for d := 0; d < 7; d++ {
		seedWorkingHour(t, db, 1, d, "08:00", "18:00")
	}

	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, days-1)

	atomic.StoreInt64(counter, 0)
	w := do(r, http.MethodGet,
		"/api/reservation/me/metrics?from="+from.Format("2006-01-02")+"&to="+to.Format("2006-01-02"),
		roleToken(t, 2, 1, "personnel"), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return atomic.LoadInt64(counter)
}

// TestPersonnelMetrics_QueryCountDoesNotScaleWithRange proves the new
// previous/occupancy blocks stay constant: a 90-day range costs exactly the
// same number of SELECTs as a single day (no per-day working-hours lookup, no
// per-day occupancy query).
func TestPersonnelMetrics_QueryCountDoesNotScaleWithRange(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	oneDay := metricsSelectCountRange(t, 1)
	ninetyDays := metricsSelectCountRange(t, 90)

	t.Logf("personnel metrics SELECTs — 1 day: %d, 90 days: %d", oneDay, ninetyDays)
	assert.Equal(t, oneDay, ninetyDays, "query count must not scale with the number of days")
	assert.LessOrEqualf(t, ninetyDays, int64(6), "constant budget, got %d SELECTs", ninetyDays)
}
