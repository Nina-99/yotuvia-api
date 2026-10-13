package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ──────────────────────────────────────────────
//  Update — the anti-flatten rule + per-service plan
// ──────────────────────────────────────────────

// updateResBody builds an assignment-free PUT /api/reservation/:id body for the
// shared two-service fixtures (services 10 and 11, 30 min each, start 10:00).
func updateResBody(personnelID uint, serviceList string) string {
	return fmt.Sprintf(
		`{"beauty_center_id":1,"personnel_id":%d,"service_ids":[%s],"date":"`+bookingTestDateKey+`","start_time":"10:00"}`,
		personnelID, serviceList,
	)
}

// updatePlanBody builds a PUT body carrying the OPTIONAL multi-professional
// plan, mirroring the create-path shape.
func updatePlanBody(serviceIDs [2]uint, personnelIDs [2]uint) string {
	return fmt.Sprintf(
		`{"beauty_center_id":1,"service_ids":[%d,%d],"assignments":[{"service_id":%d,"personnel_id":%d},{"service_id":%d,"personnel_id":%d}],"date":"`+bookingTestDateKey+`","start_time":"10:00"}`,
		serviceIDs[0], serviceIDs[1],
		serviceIDs[0], personnelIDs[0], serviceIDs[1], personnelIDs[1],
	)
}

// TestUpdateReservation_MultiProfessionalNoPlanPreservesSplit is THE anti-flatten
// guard: editing a multi-professional reservation WITHOUT a plan must leave every
// retained service line on its own professional. Under the old collapsing
// behaviour both lines would land on the requested personnel_id.
func TestUpdateReservation_MultiProfessionalNoPlanPreservesSplit(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Multi", "Client")
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)

	target := seedMultiProfessionalReservation(t, db, 1, 1, mpDay, "10:00", "11:00", []mpLine{
		{serviceID: 10, personnelID: 1, duration: 30},
		{serviceID: 11, personnelID: 2, duration: 30},
	})

	// personnel_id=2 would flatten both lines onto pro 2 under the old rule; the
	// split must survive the edit.
	w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/%d", target.ID),
		roleToken(t, 7, 1, "beauty_center"), updateResBody(2, "10,11"))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	lines := loadReservationLines(t, db, target.ID)
	require.Len(t, lines, 2)
	require.NotNil(t, lines[0].PersonnelID)
	require.NotNil(t, lines[1].PersonnelID)
	assert.Equal(t, uint(1), *lines[0].PersonnelID, "retained service 10 keeps pro 1")
	assert.Equal(t, uint(2), *lines[1].PersonnelID, "retained service 11 keeps pro 2")

	var stored models.Reservation
	require.NoError(t, db.First(&stored, target.ID).Error)
	assert.Equal(t, uint(1), stored.PersonnelID,
		"an assignment-free edit preserves the stored lead (no flattening)")
}

// TestUpdateReservation_SingleProfessionalNoPlanReassignsAllLines proves the
// historical behaviour is unchanged: when every stored line shares one
// professional, the requested personnel_id becomes the lead AND every line.
func TestUpdateReservation_SingleProfessionalNoPlanReassignsAllLines(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Single", "Client")
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)

	target := seedMultiProfessionalReservation(t, db, 1, 1, mpDay, "10:00", "11:00", []mpLine{
		{serviceID: 10, personnelID: 1, duration: 30},
		{serviceID: 11, personnelID: 1, duration: 30},
	})

	w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/%d", target.ID),
		roleToken(t, 7, 1, "beauty_center"), updateResBody(2, "10,11"))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	lines := loadReservationLines(t, db, target.ID)
	require.Len(t, lines, 2)
	for i, line := range lines {
		require.NotNilf(t, line.PersonnelID, "line %d must keep a professional", i)
		assert.Equalf(t, uint(2), *line.PersonnelID, "line %d must follow the reassignment", i)
	}
}

// TestUpdateReservation_PlanHonoured proves a supplied plan is authoritative on
// update exactly like on create: each line takes its plan professional and the
// lead is the first service's professional.
func TestUpdateReservation_PlanHonoured(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Plan", "Client")
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)

	// Stored single-professional booking...
	target := seedMultiProfessionalReservation(t, db, 1, 1, mpDay, "10:00", "11:00", []mpLine{
		{serviceID: 10, personnelID: 1, duration: 30},
		{serviceID: 11, personnelID: 1, duration: 30},
	})

	// ...edited with a two-professional plan.
	w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/%d", target.ID),
		roleToken(t, 7, 1, "beauty_center"), updatePlanBody([2]uint{10, 11}, [2]uint{2, 1}))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	lines := loadReservationLines(t, db, target.ID)
	require.Len(t, lines, 2)
	assert.Equal(t, uint(10), lines[0].ServiceID, "lines follow service_ids order")
	assert.Equal(t, uint(11), lines[1].ServiceID)
	require.NotNil(t, lines[0].PersonnelID)
	require.NotNil(t, lines[1].PersonnelID)
	assert.Equal(t, uint(2), *lines[0].PersonnelID, "service 10 → pro 2")
	assert.Equal(t, uint(1), *lines[1].PersonnelID, "service 11 → pro 1")

	var stored models.Reservation
	require.NoError(t, db.First(&stored, target.ID).Error)
	assert.Equal(t, uint(2), stored.PersonnelID, "lead = first service's professional")
}

// TestUpdateReservation_PlanValidation covers the 400 contract for a malformed
// or uncovered plan on update — identical to create.
func TestUpdateReservation_PlanValidation(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	cases := []struct {
		name string
		body string
	}{
		{
			name: "missing a requested service",
			body: `{"beauty_center_id":1,"service_ids":[10,11],"assignments":[{"service_id":10,"personnel_id":1}],"date":"` + bookingTestDateKey + `","start_time":"10:00"}`,
		},
		{
			name: "duplicate service",
			body: `{"beauty_center_id":1,"service_ids":[10,11],"assignments":[{"service_id":10,"personnel_id":1},{"service_id":10,"personnel_id":2}],"date":"` + bookingTestDateKey + `","start_time":"10:00"}`,
		},
		{
			name: "unknown service (not requested)",
			body: `{"beauty_center_id":1,"service_ids":[10,11],"assignments":[{"service_id":10,"personnel_id":1},{"service_id":99,"personnel_id":2}],"date":"` + bookingTestDateKey + `","start_time":"10:00"}`,
		},
		{
			name: "zero professional",
			body: `{"beauty_center_id":1,"service_ids":[10,11],"assignments":[{"service_id":10,"personnel_id":0},{"service_id":11,"personnel_id":1}],"date":"` + bookingTestDateKey + `","start_time":"10:00"}`,
		},
		{
			name: "zero service",
			body: `{"beauty_center_id":1,"service_ids":[10,11],"assignments":[{"service_id":0,"personnel_id":1},{"service_id":11,"personnel_id":1}],"date":"` + bookingTestDateKey + `","start_time":"10:00"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, db := multiProfessionalRouter(t)
			seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
			seedDashboardPersonnel(t, db, 1, 1, true)
			seedDashboardPersonnel(t, db, 2, 1, true)
			seedDashboardClient(t, db, 1, "Plan", "Client")
			seedServiceRow(t, db, 10, 1, 30)
			seedServiceRow(t, db, 11, 1, 30)
			target := seedMultiProfessionalReservation(t, db, 1, 1, mpDay, "10:00", "11:00", []mpLine{
				{serviceID: 10, personnelID: 1, duration: 30},
				{serviceID: 11, personnelID: 1, duration: 30},
			})

			w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/%d", target.ID),
				roleToken(t, 7, 1, "beauty_center"), tc.body)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}

// TestUpdateReservation_PlanCapabilityPerLine proves the capability rule is
// applied PER LINE on update, exactly like create.
func TestUpdateReservation_PlanCapabilityPerLine(t *testing.T) {
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
			// valid per line though the lead alone would not cover service 11.
			name:        "per-line plan accepted",
			serviceIDs:  [2]uint{10, 11},
			personnelID: [2]uint{1, 2},
			want:        http.StatusOK,
		},
		{
			// pro 1 covers only {10}: assigning it service 11 is a non-lead
			// violation a lead-only check would wrongly accept.
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
			seedDashboardClient(t, db, 1, "Plan", "Client")
			seedServiceRow(t, db, 10, 1, 30)
			seedServiceRow(t, db, 11, 1, 30)
			require.NoError(t, db.Create(&models.PersonnelService{PersonnelID: 1, ServiceID: 10}).Error)
			target := seedMultiProfessionalReservation(t, db, 1, 1, mpDay, "10:00", "11:00", []mpLine{
				{serviceID: 10, personnelID: 1, duration: 30},
				{serviceID: 11, personnelID: 1, duration: 30},
			})

			w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/%d", target.ID),
				roleToken(t, 7, 1, "beauty_center"), updatePlanBody(tc.serviceIDs, tc.personnelID))
			assert.Equal(t, tc.want, w.Code, w.Body.String())
		})
	}
}

// TestUpdateReservation_PlanOverlapPerProfessional proves the update plan
// reuses the Phase 3a segment overlap helper: a professional whose NEW chained
// segment would conflict is rejected, while another professional at the same
// wall clock is accepted.
func TestUpdateReservation_PlanOverlapPerProfessional(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	t.Run("conflicting professional rejected", func(t *testing.T) {
		r, db := multiProfessionalRouter(t)
		seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
		seedDashboardPersonnel(t, db, 1, 1, true)
		seedDashboardPersonnel(t, db, 2, 1, true)
		seedDashboardClient(t, db, 1, "Plan", "Client")
		seedServiceRow(t, db, 10, 1, 30)
		seedServiceRow(t, db, 11, 1, 30)
		target := seedMultiProfessionalReservation(t, db, 1, 1, mpDay, "09:00", "10:00", []mpLine{
			{serviceID: 10, personnelID: 1, duration: 30},
			{serviceID: 11, personnelID: 2, duration: 30},
		})
		// pro 2 is busy [10:30,11:00); the new plan puts service 11 (pro 2) there.
		seedReservationRow(t, db, 1, 2, mpDay, "10:30", "11:00")

		w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/%d", target.ID),
			roleToken(t, 7, 1, "beauty_center"), updatePlanBody([2]uint{10, 11}, [2]uint{1, 2}))
		assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	})

	t.Run("same wall clock accepted for a free professional", func(t *testing.T) {
		r, db := multiProfessionalRouter(t)
		seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
		seedDashboardPersonnel(t, db, 1, 1, true)
		seedDashboardPersonnel(t, db, 2, 1, true)
		seedDashboardClient(t, db, 1, "Plan", "Client")
		seedServiceRow(t, db, 10, 1, 30)
		seedServiceRow(t, db, 11, 1, 30)
		target := seedMultiProfessionalReservation(t, db, 1, 1, mpDay, "09:00", "10:00", []mpLine{
			{serviceID: 10, personnelID: 1, duration: 30},
			{serviceID: 11, personnelID: 2, duration: 30},
		})
		seedReservationRow(t, db, 1, 2, mpDay, "10:30", "11:00")

		// Both services on pro 1: no segment overlaps pro 2's booking.
		w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/%d", target.ID),
			roleToken(t, 7, 1, "beauty_center"), updatePlanBody([2]uint{10, 11}, [2]uint{1, 1}))
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	})
}

// TestUpdateReservation_NewServiceTakesRequestedProfessional covers the
// newly-added-service rule of the anti-flatten path: a retained service keeps
// its professional, a brand-new service takes the requested personnel_id (or
// the reservation's existing lead when none was given).
func TestUpdateReservation_NewServiceTakesRequestedProfessional(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	t.Run("requested personnel_id", func(t *testing.T) {
		r, db := multiProfessionalRouter(t)
		seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
		seedDashboardPersonnel(t, db, 1, 1, true)
		seedDashboardPersonnel(t, db, 2, 1, true)
		seedDashboardPersonnel(t, db, 3, 1, true)
		seedDashboardClient(t, db, 1, "Grow", "Client")
		seedServiceRow(t, db, 10, 1, 30)
		seedServiceRow(t, db, 11, 1, 30)
		seedServiceRow(t, db, 12, 1, 30)
		target := seedMultiProfessionalReservation(t, db, 1, 1, mpDay, "10:00", "11:00", []mpLine{
			{serviceID: 10, personnelID: 1, duration: 30},
			{serviceID: 11, personnelID: 2, duration: 30},
		})

		// Keep 10, drop 11, add new service 12; ask for pro 3.
		body := `{"beauty_center_id":1,"personnel_id":3,"service_ids":[10,12],"date":"` + bookingTestDateKey + `","start_time":"10:00"}`
		w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/%d", target.ID),
			roleToken(t, 7, 1, "beauty_center"), body)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		lines := loadReservationLines(t, db, target.ID)
		require.Len(t, lines, 2)
		require.NotNil(t, lines[0].PersonnelID)
		require.NotNil(t, lines[1].PersonnelID)
		assert.Equal(t, uint(1), *lines[0].PersonnelID, "retained service 10 keeps pro 1")
		assert.Equal(t, uint(3), *lines[1].PersonnelID, "new service 12 takes the requested pro 3")
	})

	t.Run("falls back to the existing lead", func(t *testing.T) {
		r, db := multiProfessionalRouter(t)
		seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
		seedDashboardPersonnel(t, db, 1, 1, true)
		seedDashboardPersonnel(t, db, 2, 1, true)
		seedDashboardClient(t, db, 1, "Grow", "Client")
		seedServiceRow(t, db, 10, 1, 30)
		seedServiceRow(t, db, 11, 1, 30)
		seedServiceRow(t, db, 12, 1, 30)
		target := seedMultiProfessionalReservation(t, db, 1, 1, mpDay, "10:00", "11:00", []mpLine{
			{serviceID: 10, personnelID: 1, duration: 30},
			{serviceID: 11, personnelID: 2, duration: 30},
		})

		// No top-level personnel_id: the new service falls back to the stored lead.
		body := `{"beauty_center_id":1,"personnel_id":0,"service_ids":[10,12],"date":"` + bookingTestDateKey + `","start_time":"10:00"}`
		w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/%d", target.ID),
			roleToken(t, 7, 1, "beauty_center"), body)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		lines := loadReservationLines(t, db, target.ID)
		require.Len(t, lines, 2)
		require.NotNil(t, lines[0].PersonnelID)
		require.NotNil(t, lines[1].PersonnelID)
		assert.Equal(t, uint(1), *lines[0].PersonnelID, "retained service 10 keeps pro 1")
		assert.Equal(t, uint(1), *lines[1].PersonnelID, "new service 12 falls back to the stored lead")
	})
}
