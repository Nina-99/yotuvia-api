package tests

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ──────────────────────────────────────────────
//  Phase 3a — per-service professional assignments + chained availability
// ──────────────────────────────────────────────

// assignmentBody builds a client booking body with a two-professional plan.
func assignmentBody(serviceIDs [2]uint, personnelIDs [2]uint, start string) string {
	return fmt.Sprintf(
		`{"beauty_center_id":1,"service_ids":[%d,%d],"assignments":[{"service_id":%d,"personnel_id":%d},{"service_id":%d,"personnel_id":%d}],"date":"`+bookingTestDateKey+`","start_time":%q}`,
		serviceIDs[0], serviceIDs[1],
		serviceIDs[0], personnelIDs[0], serviceIDs[1], personnelIDs[1],
		start,
	)
}

// TestCreateReservation_MultiProfessionalAssignments pins the create contract:
// when assignments are present every line persists its OWN professional, the
// derived lead is the professional of the FIRST service in service_ids order,
// and the envelope stays StartTime + Σ durations.
func TestCreateReservation_MultiProfessionalAssignments(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Chain", "Client")
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)
	seedWorkingHour(t, db, 1, int(mpDay.Weekday()), "08:00", "13:00")

	body := assignmentBody([2]uint{10, 11}, [2]uint{1, 2}, "09:00")
	w := do(r, http.MethodPost, "/api/reservation/", roleToken(t, 1, 0, "client"), body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var created models.Reservation
	require.NoError(t, db.Where("client_id = ? AND start_time = ?", 1, "09:00").First(&created).Error)
	assert.Equal(t, uint(1), created.PersonnelID, "lead = professional of the FIRST service in service_ids order")
	assert.Equal(t, "10:00", created.EndTime, "envelope = start + Σ durations (unchanged)")

	lines := loadReservationLines(t, db, created.ID)
	require.Len(t, lines, 2)
	assert.Equal(t, uint(10), lines[0].ServiceID, "lines follow service_ids order")
	assert.Equal(t, uint(11), lines[1].ServiceID)
	require.NotNil(t, lines[0].PersonnelID)
	require.NotNil(t, lines[1].PersonnelID)
	assert.Equal(t, uint(1), *lines[0].PersonnelID, "line 1 professional")
	assert.Equal(t, uint(2), *lines[1].PersonnelID, "line 2 professional")
}

// TestCreateReservation_LeadFollowsServiceIDsOrder proves the lead is derived
// from service_ids ORDER, not from the assignments array order: reversing the
// request order flips the lead while the plan still covers the same services.
func TestCreateReservation_LeadFollowsServiceIDsOrder(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Chain", "Client")
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)
	seedWorkingHour(t, db, 1, int(mpDay.Weekday()), "08:00", "13:00")

	// service_ids = [11,10] but the plan still binds 10→1 and 11→2. The lead is
	// the professional of service 11 (first requested) = 2.
	body := `{"beauty_center_id":1,"service_ids":[11,10],"assignments":[{"service_id":10,"personnel_id":1},{"service_id":11,"personnel_id":2}],"date":"` + bookingTestDateKey + `","start_time":"09:00"}`
	w := do(r, http.MethodPost, "/api/reservation/", roleToken(t, 1, 0, "client"), body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var created models.Reservation
	require.NoError(t, db.Where("client_id = ? AND start_time = ?", 1, "09:00").First(&created).Error)
	assert.Equal(t, uint(2), created.PersonnelID, "lead = professional of the first REQUESTED service (11→2)")

	lines := loadReservationLines(t, db, created.ID)
	require.Len(t, lines, 2)
	assert.Equal(t, uint(11), lines[0].ServiceID)
	assert.Equal(t, uint(10), lines[1].ServiceID)
	require.NotNil(t, lines[0].PersonnelID)
	require.NotNil(t, lines[1].PersonnelID)
	assert.Equal(t, uint(2), *lines[0].PersonnelID)
	assert.Equal(t, uint(1), *lines[1].PersonnelID)
}

// TestCreateReservation_AssignmentCapabilityPerLine proves the Phase 0
// capability rule is applied PER LINE, not just to the lead. The lead-only
// check would accept the negative case and reject the positive one; only a
// per-line check gets both right.
func TestCreateReservation_AssignmentCapabilityPerLine(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	cases := []struct {
		name        string
		serviceIDs  [2]uint
		personnelID [2]uint
		want        int
	}{
		{
			// pro 1 is restricted to {10}; pro 2 is unrestricted. The plan is
			// valid per line, but the LEAD (pro 1) does NOT cover service 11 —
			// a lead-only check would wrongly 400.
			name:        "per-line plan accepted though the lead alone does not cover every service",
			serviceIDs:  [2]uint{10, 11},
			personnelID: [2]uint{1, 2},
			want:        http.StatusCreated,
		},
		{
			// pro 2 is unrestricted and is the lead, but pro 1 (restricted to
			// {10}) is assigned service 11 — a non-lead violation a lead-only
			// check would wrongly ACCEPT.
			name:        "non-lead line violation rejected",
			serviceIDs:  [2]uint{10, 11},
			personnelID: [2]uint{2, 1},
			want:        http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, db := multiProfessionalRouter(t)
			seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
			seedDashboardPersonnel(t, db, 1, 1, true)
			seedDashboardPersonnel(t, db, 2, 1, true)
			seedDashboardClient(t, db, 1, "Chain", "Client")
			seedServiceRow(t, db, 10, 1, 30)
			seedServiceRow(t, db, 11, 1, 30)
			seedWorkingHour(t, db, 1, int(mpDay.Weekday()), "08:00", "13:00")
			// pro 1 may only perform service 10; pro 2 has NO rows (unrestricted).
			require.NoError(t, db.Create(&models.PersonnelService{PersonnelID: 1, ServiceID: 10}).Error)

			body := assignmentBody(tc.serviceIDs, tc.personnelID, "09:00")
			w := do(r, http.MethodPost, "/api/reservation/", roleToken(t, 1, 0, "client"), body)
			assert.Equal(t, tc.want, w.Code, w.Body.String())
		})
	}
}

// TestCreateReservation_AssignmentPlanValidation covers the 400 contract: the
// plan must cover service_ids exactly once with non-zero ids.
func TestCreateReservation_AssignmentPlanValidation(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	cases := []struct {
		name string
		body string
	}{
		{
			name: "missing a requested service",
			body: `{"beauty_center_id":1,"service_ids":[10,11],"assignments":[{"service_id":10,"personnel_id":1}],"date":"` + bookingTestDateKey + `","start_time":"09:00"}`,
		},
		{
			name: "duplicate service",
			body: `{"beauty_center_id":1,"service_ids":[10,11],"assignments":[{"service_id":10,"personnel_id":1},{"service_id":10,"personnel_id":2}],"date":"` + bookingTestDateKey + `","start_time":"09:00"}`,
		},
		{
			name: "unknown service (not requested)",
			body: `{"beauty_center_id":1,"service_ids":[10,11],"assignments":[{"service_id":10,"personnel_id":1},{"service_id":99,"personnel_id":2}],"date":"` + bookingTestDateKey + `","start_time":"09:00"}`,
		},
		{
			name: "zero professional",
			body: `{"beauty_center_id":1,"service_ids":[10,11],"assignments":[{"service_id":10,"personnel_id":0},{"service_id":11,"personnel_id":1}],"date":"` + bookingTestDateKey + `","start_time":"09:00"}`,
		},
		{
			name: "zero service",
			body: `{"beauty_center_id":1,"service_ids":[10,11],"assignments":[{"service_id":0,"personnel_id":1},{"service_id":11,"personnel_id":1}],"date":"` + bookingTestDateKey + `","start_time":"09:00"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, db := multiProfessionalRouter(t)
			seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
			seedDashboardPersonnel(t, db, 1, 1, true)
			seedDashboardClient(t, db, 1, "Chain", "Client")
			seedServiceRow(t, db, 10, 1, 30)
			seedServiceRow(t, db, 11, 1, 30)
			seedWorkingHour(t, db, 1, int(mpDay.Weekday()), "08:00", "13:00")

			w := do(r, http.MethodPost, "/api/reservation/", roleToken(t, 1, 0, "client"), tc.body)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}

// ──────────────────────────────────────────────
//  The acceptance test: search ⟺ booking agreement for a chain
// ──────────────────────────────────────────────

// TestAvailabilitySearch_Chain_Agreement is THE acceptance guard for Phase 3a.
// For a chained plan [(10→pro 1),(11→pro 2)] against fixtures where pro 1 is
// busy at t=10:00 and pro 2 is busy for its chained segment at t=10:30:
//
//   - the search EXCLUDES 10:00 (pro 1 busy) and 10:30 (pro 2 busy) and
//     INCLUDES 11:00 (both free);
//   - a booking at an INCLUDED slot is 201;
//   - a booking at an EXCLUDED slot is 409.
//
// So a returned slot is bookable and an excluded one is rejected.
func TestAvailabilitySearch_Chain_Agreement(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Chain", "Client")
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)
	seedWorkingHour(t, db, 1, int(mpDay.Weekday()), "08:00", "13:00")

	// pro 1 busy [10:00,10:30) — blocks t=10:00; pro 2 busy [11:00,11:30) —
	// blocks t=10:30 (pro 2's chained segment is [t+30, t+60)).
	seedReservationRow(t, db, 1, 1, mpDay, "10:00", "10:30")
	seedReservationRow(t, db, 1, 2, mpDay, "11:00", "11:30")

	chainQuery := "service_ids=10,11&assignments=10:1,11:2&from=" + bookingTestDateKey + "&to=" + bookingTestDateKey + "&center_id=1&limit=50"
	times := searchStartTimes(getSearch(t, r, chainQuery))
	assert.NotContains(t, times, "10:00", "pro 1 is busy at the candidate start")
	assert.NotContains(t, times, "10:30", "pro 2 is busy at its chained segment")
	assert.Contains(t, times, "11:00", "both professionals are free later")

	client := roleToken(t, 1, 0, "client")

	included := do(r, http.MethodPost, "/api/reservation/", client,
		assignmentBody([2]uint{10, 11}, [2]uint{1, 2}, "11:00"))
	assert.Equal(t, http.StatusCreated, included.Code, "an offered slot must be bookable: %s", included.Body.String())

	excluded := do(r, http.MethodPost, "/api/reservation/", client,
		assignmentBody([2]uint{10, 11}, [2]uint{1, 2}, "10:00"))
	assert.Equal(t, http.StatusConflict, excluded.Code, "a slot the search excluded must be rejected: %s", excluded.Body.String())
}

// TestAvailabilitySearch_Chain_SingleProfessionalControl extends (does not
// replace) the existing search==per-center consistency coverage: a ONE-step
// chain reduces exactly to the legacy professional-scoped path, and the
// assignment-free legacy query is untouched.
func TestAvailabilitySearch_Chain_SingleProfessionalControl(t *testing.T) {
	r, db := availabilityTestRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 5, 1, true)
	seedServiceRow(t, db, 10, 1, 30)
	seedWorkingHour(t, db, 1, int(mpDay.Weekday()), "08:00", "13:00")
	seedReservationRow(t, db, 1, 5, mpDay, "10:00", "11:00")

	perCenter := getPerCenterAvailable(t, r,
		"/api/beauty-centers/1/available-slots?date="+bookingTestDateKey+"&duration=30&personnel_id=5")

	chain := getSearch(t, r, "service_ids=10&assignments=10:5&from="+bookingTestDateKey+"&to="+bookingTestDateKey+"&center_id=1&limit=50")
	assert.Equal(t, perCenter, searchStartTimes(chain), "a single-step chain equals the per-center path")

	legacy := getSearch(t, r, "service_ids=10&personnel_id=5&from="+bookingTestDateKey+"&to="+bookingTestDateKey+"&center_id=1&limit=50")
	assert.Equal(t, perCenter, searchStartTimes(legacy), "the assignment-free legacy path is untouched")
}

// TestAvailabilitySearch_Chain_Validation covers the 400 contract for a
// malformed or uncovered plan, and proves an absent plan is still accepted.
func TestAvailabilitySearch_Chain_Validation(t *testing.T) {
	r, db := availabilityTestRouter(t)
	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)
	seedWorkingHour(t, db, 1, int(mpDay.Weekday()), "08:00", "13:00")

	cases := []struct {
		name  string
		query string
	}{
		{"missing coverage", "service_ids=10,11&assignments=10:1&from=" + bookingTestDateKey + "&to=" + bookingTestDateKey + ""},
		{"duplicate service", "service_ids=10,11&assignments=10:1,10:2&from=" + bookingTestDateKey + "&to=" + bookingTestDateKey + ""},
		{"unknown service", "service_ids=10,11&assignments=10:1,99:2&from=" + bookingTestDateKey + "&to=" + bookingTestDateKey + ""},
		{"unknown personnel", "service_ids=10,11&assignments=10:1,11:999&from=" + bookingTestDateKey + "&to=" + bookingTestDateKey + ""},
		{"zero personnel", "service_ids=10,11&assignments=10:0,11:1&from=" + bookingTestDateKey + "&to=" + bookingTestDateKey + ""},
		{"malformed pair", "service_ids=10,11&assignments=10-1,11:2&from=" + bookingTestDateKey + "&to=" + bookingTestDateKey + ""},
		{"chain with treatment_ids", "service_ids=10&treatment_ids=5&assignments=10:1&from=" + bookingTestDateKey + "&to=" + bookingTestDateKey + ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := do(r, http.MethodGet, "/api/availability/search?"+tc.query, "", "")
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}

	// An absent plan keeps the legacy contract intact.
	w := do(r, http.MethodGet,
		"/api/availability/search?service_ids=10&from="+bookingTestDateKey+"&to="+bookingTestDateKey+"&center_id=1", "", "")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// ──────────────────────────────────────────────
//  Query budget — the chain must not N+1
// ──────────────────────────────────────────────

// TestAvailabilitySearch_Chain_QueryBudget proves the chain evaluation adds NO
// per-professional or per-slot query: candidate centers, work hours and the
// chain-scoped reservations load in a bounded number of SELECTs, and each
// professional is projected in memory.
func TestAvailabilitySearch_Chain_QueryBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	counter := attachQueryCounter(db)

	r := gin.New()
	api := r.Group("/api")
	router.AvailabilityRoutes(api, db, validator.New())
	router.BeautyCenterRoutes(api, db, validator.New())

	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)

	const centers, days = 5, 7
	for c := uint(1); c <= centers; c++ {
		seedDashboardCenter(t, db, c, fmt.Sprintf("Center %d", c), fmt.Sprintf("c%d@test.com", c), true)
		for d := 0; d < 7; d++ {
			seedWorkingHour(t, db, c, d, "08:00", "20:00")
		}
		seedReservationRow(t, db, c, 1, mpDay, "10:00", "11:00")
		seedReservationRow(t, db, c, 2, mpDay, "12:00", "13:00")
	}

	w := do(r, http.MethodGet,
		"/api/availability/search?service_ids=10,11&assignments=10:1,11:2&from="+bookingTestDateKey+"&to="+bookingTestDatePlus(6)+"&limit=50", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	queries := atomic.LoadInt64(counter)
	t.Logf("chained availability search SELECTs — %d centers x %d days: %d", centers, days, queries)

	assert.Lessf(t, queries, int64(10),
		"the chain must use a bounded number of SELECTs (no per-professional or per-slot query), got %d", queries)
}
