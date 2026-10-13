package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  Treatment-driven cross-center availability search
// ──────────────────────────────────────────────

func seedTreatmentCategoryRow(t *testing.T, db *gorm.DB, id uint) {
	t.Helper()
	require.NoError(t, db.Create(&models.TreatmentCategory{
		ID: id, Name: fmt.Sprintf("cat-%d", id), Icon: "💇",
	}).Error)
}

func seedTreatmentRow(t *testing.T, db *gorm.DB, id, categoryID uint, name string) {
	t.Helper()
	require.NoError(t, db.Create(&models.Treatment{
		ID: id, TreatmentCategoryID: categoryID, Name: name, IsActive: true,
	}).Error)
}

// seedTreatmentService creates a per-center service linked to a global treatment.
func seedTreatmentService(t *testing.T, db *gorm.DB, id, centerID, treatmentID uint, duration int, active bool) {
	t.Helper()
	tid := treatmentID
	require.NoError(t, db.Create(&models.Service{
		ID: id, BeautyCenterID: centerID, CategoryID: 1, Name: fmt.Sprintf("t-%d", id),
		Description: "d", Duration: duration, Price: 50, IsActive: active, TreatmentID: &tid,
	}).Error)
	// gorm:"default:true" coerces a zero-value IsActive back to true on insert,
	// so force the explicit value for inactive seeds.
	if !active {
		require.NoError(t, db.Model(&models.Service{}).Where("id = ?", id).Update("is_active", false).Error)
	}
}

// TestAvailabilitySearch_TreatmentDriven_ExcludesAndIncludes is the core honesty
// guarantee: a center with NO active service for the requested treatment must be
// excluded, and the window must be sized from the center's OWN service duration.
func TestAvailabilitySearch_TreatmentDriven_ExcludesAndIncludes(t *testing.T) {
	r, db := availabilityTestRouter(t)

	seedDashboardCenter(t, db, 1, "Has Treatment", "c1@test.com", true)
	seedDashboardCenter(t, db, 2, "No Treatment", "c2@test.com", true)
	seedTreatmentCategoryRow(t, db, 1)
	seedTreatmentRow(t, db, 100, 1, "Corte de cabello")
	seedTreatmentService(t, db, 10, 1, 100, 30, true)

	date := bookingTestDate // Monday
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")
	seedWorkingHour(t, db, 2, int(date.Weekday()), "09:00", "18:00")

	env := getSearch(t, r, "treatment_ids=100&from="+bookingTestDateKey+"&to="+bookingTestDateKey+"&limit=50")
	require.NotEmpty(t, env.Results)
	for _, res := range env.Results {
		assert.Equal(t, uint(1), res.CenterID, "center without the treatment must be excluded")
		assert.Equal(t, 30, res.Duration, "duration must come from the center's own service")
	}
}

// TestAvailabilitySearch_TreatmentDriven_ActiveOnly proves an INACTIVE matching
// service does not make a center a candidate.
func TestAvailabilitySearch_TreatmentDriven_ActiveOnly(t *testing.T) {
	r, db := availabilityTestRouter(t)

	seedDashboardCenter(t, db, 1, "Inactive Only", "c1@test.com", true)
	seedTreatmentCategoryRow(t, db, 1)
	seedTreatmentRow(t, db, 100, 1, "Corte de cabello")
	seedTreatmentService(t, db, 10, 1, 100, 30, false) // inactive

	date := bookingTestDate
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")

	env := getSearch(t, r, "treatment_ids=100&from="+bookingTestDateKey+"&to="+bookingTestDateKey+"&limit=50")
	assert.Empty(t, env.Results)
	assert.Equal(t, 0, env.Total)
}

// TestAvailabilitySearch_TreatmentDriven_MultipleMatchesUsesMin documents the
// chosen rule: when a center offers several services for one treatment, the
// MINIMUM active duration sizes the window (most inclusive).
func TestAvailabilitySearch_TreatmentDriven_MultipleMatchesUsesMin(t *testing.T) {
	r, db := availabilityTestRouter(t)

	seedDashboardCenter(t, db, 1, "Variants", "c1@test.com", true)
	seedTreatmentCategoryRow(t, db, 1)
	seedTreatmentRow(t, db, 100, 1, "Corte de cabello")
	seedTreatmentService(t, db, 10, 1, 100, 45, true)
	seedTreatmentService(t, db, 11, 1, 100, 20, true)

	date := bookingTestDate
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")

	env := getSearch(t, r, "treatment_ids=100&from="+bookingTestDateKey+"&to="+bookingTestDateKey+"&limit=50")
	require.NotEmpty(t, env.Results)
	assert.Equal(t, 20, env.Results[0].Duration, "minimum matching duration wins")
}

// TestAvailabilitySearch_TreatmentDriven_MultipleTreatmentsSumsAndRequiresAll
// documents the combined rule: a center must offer EVERY requested treatment,
// and their per-center minima are summed (mirrors the booking flow).
func TestAvailabilitySearch_TreatmentDriven_MultipleTreatmentsSumsAndRequiresAll(t *testing.T) {
	r, db := availabilityTestRouter(t)

	seedDashboardCenter(t, db, 1, "Both", "c1@test.com", true)
	seedDashboardCenter(t, db, 2, "Only Hair", "c2@test.com", true)
	seedDashboardCenter(t, db, 3, "Only Nails", "c3@test.com", true)
	seedTreatmentCategoryRow(t, db, 1)
	seedTreatmentRow(t, db, 100, 1, "Corte de cabello")
	seedTreatmentRow(t, db, 200, 1, "Manicuría semipermanente")

	seedTreatmentService(t, db, 10, 1, 100, 30, true)
	seedTreatmentService(t, db, 11, 1, 200, 45, true)
	seedTreatmentService(t, db, 12, 2, 100, 60, true)
	seedTreatmentService(t, db, 13, 3, 200, 60, true)

	date := bookingTestDate
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")
	seedWorkingHour(t, db, 2, int(date.Weekday()), "09:00", "18:00")
	seedWorkingHour(t, db, 3, int(date.Weekday()), "09:00", "18:00")

	env := getSearch(t, r, "treatment_ids=100,200&from="+bookingTestDateKey+"&to="+bookingTestDateKey+"&limit=50")
	require.NotEmpty(t, env.Results)

	seen := map[uint]bool{}
	for _, res := range env.Results {
		seen[res.CenterID] = true
		assert.Equal(t, 75, res.Duration, "combined duration = 30 + 45")
	}
	assert.True(t, seen[1], "center offering both treatments must be included")
	assert.False(t, seen[2], "center missing one treatment must be excluded")
	assert.False(t, seen[3], "center missing one treatment must be excluded")
}

// TestAvailabilitySearch_TreatmentPrecedenceOverServiceIDs documents precedence:
// when both are supplied, treatment_ids wins and service_ids is ignored.
func TestAvailabilitySearch_TreatmentPrecedenceOverServiceIDs(t *testing.T) {
	r, db := availabilityTestRouter(t)

	seedDashboardCenter(t, db, 1, "Center One", "c1@test.com", true)
	seedDashboardCenter(t, db, 2, "Center Two", "c2@test.com", true)
	seedTreatmentCategoryRow(t, db, 1)
	seedTreatmentRow(t, db, 100, 1, "Corte de cabello")
	// Treatment-linked service at center 1: duration 30.
	seedTreatmentService(t, db, 10, 1, 100, 30, true)
	// Legacy unlinked services: durations would sum to 80 and include center 2.
	seedServiceRow(t, db, 11, 1, 40)
	seedServiceRow(t, db, 12, 2, 40)

	date := bookingTestDate
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")
	seedWorkingHour(t, db, 2, int(date.Weekday()), "09:00", "18:00")

	env := getSearch(t, r, "service_ids=11,12&treatment_ids=100&from="+bookingTestDateKey+"&to="+bookingTestDateKey+"&limit=50")
	require.NotEmpty(t, env.Results)
	for _, res := range env.Results {
		assert.Equal(t, uint(1), res.CenterID, "service_ids must NOT widen the candidate set")
		assert.Equal(t, 30, res.Duration, "duration must come from the treatment path, not service_ids")
	}
}

// TestAvailabilitySearch_TreatmentDriven_ConsistentWithPerCenter extends the
// acceptance guard to the treatment path: the search must agree EXACTLY with the
// per-center endpoint for the same (center, date, duration, personnel), where
// duration is the center's own matching service duration.
func TestAvailabilitySearch_TreatmentDriven_ConsistentWithPerCenter(t *testing.T) {
	r, db := availabilityTestRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedTreatmentCategoryRow(t, db, 1)
	seedTreatmentRow(t, db, 100, 1, "Corte de cabello")
	seedTreatmentService(t, db, 10, 1, 100, 45, true)

	date := bookingTestDate // Monday
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")
	seedReservationRow(t, db, 1, 5, date, "10:00", "11:00")
	seedReservationRow(t, db, 1, 7, date, "14:00", "15:00")

	for _, personnelID := range []uint{5, 0, 9} {
		query := fmt.Sprintf("treatment_ids=100&from="+bookingTestDateKey+"&to="+bookingTestDateKey+"&center_id=1&limit=50&personnel_id=%d", personnelID)
		env := getSearch(t, r, query)
		perCenter := getPerCenterAvailable(t, r,
			fmt.Sprintf("/api/beauty-centers/1/available-slots?date="+bookingTestDateKey+"&duration=45&personnel_id=%d", personnelID))
		assert.Equal(t, perCenter, searchStartTimes(env),
			"treatment-driven search must equal per-center slots (personnel=%d)", personnelID)
	}
}

// TestAvailabilitySearch_TreatmentDriven_UnknownTreatment is the 400 contract
// for an unknown/inactive treatment id, mirroring the unknown-service rule.
func TestAvailabilitySearch_TreatmentDriven_UnknownTreatment(t *testing.T) {
	r, db := availabilityTestRouter(t)
	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)

	w := do(r, http.MethodGet, "/api/availability/search?treatment_ids=999&from="+bookingTestDateKey+"&to="+bookingTestDateKey+"", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	// Neither param is still a 400.
	w = do(r, http.MethodGet, "/api/availability/search?from="+bookingTestDateKey+"&to="+bookingTestDateKey+"", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Malformed treatment_ids is a 400.
	w = do(r, http.MethodGet, "/api/availability/search?treatment_ids=abc&from="+bookingTestDateKey+"&to="+bookingTestDateKey+"", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAvailabilitySearch_TreatmentDriven_ReportsOwnServiceIDs proves a
// treatment-driven result names the CENTER'S OWN matching active service — the
// one whose duration sized the window — not anything the caller supplied.
func TestAvailabilitySearch_TreatmentDriven_ReportsOwnServiceIDs(t *testing.T) {
	r, db := availabilityTestRouter(t)

	seedDashboardCenter(t, db, 1, "Has Treatment", "c1@test.com", true)
	seedTreatmentCategoryRow(t, db, 1)
	seedTreatmentRow(t, db, 100, 1, "Corte de cabello")
	seedTreatmentService(t, db, 10, 1, 100, 30, true)

	date := bookingTestDate // Monday
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")

	env := getSearch(t, r, "treatment_ids=100&from="+bookingTestDateKey+"&to="+bookingTestDateKey+"&limit=50")
	require.NotEmpty(t, env.Results)
	for _, res := range env.Results {
		assert.Equal(t, uint(1), res.CenterID)
		assert.Equal(t, []uint{10}, res.ServiceIDs,
			"result must carry the center's own matching active service id")
		assert.Equal(t, 30, res.Duration)
	}
}

// TestAvailabilitySearch_TreatmentDriven_MultipleMatchesReportsSizingService
// documents the chosen honesty rule for several matching services: the window
// duration is the MIN, so only the SERVICE WHOSE DURATION WAS USED is reported
// (the lowest id on a tie). Reporting every variant would misstate the duration
// and break the booking-flow sum.
func TestAvailabilitySearch_TreatmentDriven_MultipleMatchesReportsSizingService(t *testing.T) {
	r, db := availabilityTestRouter(t)

	seedDashboardCenter(t, db, 1, "Variants", "c1@test.com", true)
	seedTreatmentCategoryRow(t, db, 1)
	seedTreatmentRow(t, db, 100, 1, "Corte de cabello")
	seedTreatmentService(t, db, 10, 1, 100, 45, true)
	seedTreatmentService(t, db, 11, 1, 100, 20, true) // shorter → sizes the window

	date := bookingTestDate
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")

	env := getSearch(t, r, "treatment_ids=100&from="+bookingTestDateKey+"&to="+bookingTestDateKey+"&limit=50")
	require.NotEmpty(t, env.Results)
	assert.Equal(t, 20, env.Results[0].Duration, "minimum matching duration wins")
	assert.Equal(t, []uint{11}, env.Results[0].ServiceIDs,
		"only the sizing service (min duration) is reported, not every variant")
}

// TestAvailabilitySearch_TreatmentDriven_ServiceIDsUnionAndSum proves that with
// several requested treatments the result reports the union (in request order)
// of the per-center sizing services, and their durations sum to the window — so
// the booking flow reproducing that sum lands on the searched duration.
func TestAvailabilitySearch_TreatmentDriven_ServiceIDsUnionAndSum(t *testing.T) {
	r, db := availabilityTestRouter(t)

	seedDashboardCenter(t, db, 1, "Both", "c1@test.com", true)
	seedTreatmentCategoryRow(t, db, 1)
	seedTreatmentRow(t, db, 100, 1, "Corte de cabello")
	seedTreatmentRow(t, db, 200, 1, "Manicuría semipermanente")
	seedTreatmentService(t, db, 10, 1, 100, 30, true)
	seedTreatmentService(t, db, 11, 1, 200, 45, true)

	date := bookingTestDate
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")

	env := getSearch(t, r, "treatment_ids=100,200&from="+bookingTestDateKey+"&to="+bookingTestDateKey+"&limit=50")
	require.NotEmpty(t, env.Results)
	for _, res := range env.Results {
		assert.Equal(t, []uint{10, 11}, res.ServiceIDs,
			"union of the sizing services, in request order")
		assert.Equal(t, 75, res.Duration, "sum of the reported services' durations")
	}
}

// TestAvailabilitySearch_LegacyPathOmitsServiceIDs guards the additive
// contract: the legacy service_ids path must OMIT the field entirely, because
// the caller already knows the ids and there is no center filter.
func TestAvailabilitySearch_LegacyPathOmitsServiceIDs(t *testing.T) {
	r, db := availabilityTestRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedServiceRow(t, db, 10, 1, 30)

	date := bookingTestDate
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")

	w := do(r, http.MethodGet,
		"/api/availability/search?service_ids=10&from="+bookingTestDateKey+"&to="+bookingTestDateKey+"&limit=50", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "service_ids",
		"legacy path must omit service_ids from the JSON")

	var env searchEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	require.NotEmpty(t, env.Results)
	assert.Nil(t, env.Results[0].ServiceIDs)
}

// TestAvailabilitySearch_TreatmentDriven_QueryCountIsBounded proves the
// treatment path stays batched: a 5-center x 7-day search uses a small constant
// number of SELECTs, never one per center or per day.
func TestAvailabilitySearch_TreatmentDriven_QueryCountIsBounded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	counter := attachQueryCounter(db)

	r := gin.New()
	api := r.Group("/api")
	router.AvailabilityRoutes(api, db, validator.New())
	router.BeautyCenterRoutes(api, db, validator.New())

	seedTreatmentCategoryRow(t, db, 1)
	seedTreatmentRow(t, db, 100, 1, "Corte de cabello")

	const centers, days = 5, 7
	for c := uint(1); c <= centers; c++ {
		seedDashboardCenter(t, db, c, fmt.Sprintf("Center %d", c), fmt.Sprintf("c%d@test.com", c), true)
		for d := 0; d < 7; d++ {
			seedWorkingHour(t, db, c, d, "08:00", "20:00")
		}
		seedTreatmentService(t, db, 10+c, c, 100, 30, true)
		seedReservationRow(t, db, c, 1, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), "10:00", "11:00")
	}

	atomic.StoreInt64(counter, 0)
	w := do(r, http.MethodGet,
		"/api/availability/search?treatment_ids=100&from="+bookingTestDateKey+"&to="+bookingTestDatePlus(6)+"&limit=50", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	queries := atomic.LoadInt64(counter)
	t.Logf("treatment-driven search SELECTs — %d centers x %d days: %d", centers, days, queries)

	assert.Lessf(t, queries, int64(10),
		"treatment-driven search must use a bounded number of SELECTs, got %d", queries)

	// Surfacing service_ids must NOT grow the budget: the ids are derived from
	// the services already loaded to size the windows. The treatment path is a
	// fixed 5 SELECTs (treatments, candidate centers, active services, working
	// hours, reservations); pin it so a per-row regression fails loudly.
	var env searchEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	require.NotEmpty(t, env.Results)
	assert.NotEmpty(t, env.Results[0].ServiceIDs, "service_ids must be surfaced")
	assert.LessOrEqualf(t, queries, int64(5),
		"treatment search SELECTs must not grow beyond the 5-query baseline, got %d", queries)
}
