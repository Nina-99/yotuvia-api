package tests

import (
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Chained availability must honour unavailability BLOCKS too.
//
// The chain path builds a PER-PROFESSIONAL busy set (a shared reservation can
// occupy pro 1 at [t,t+d1) and pro 2 at [t+d1,t+d2) with no conflict), so a block
// has to be folded into that same projection: a centre-wide block hits every
// professional, one professional's block only their own segment.

func TestAvailabilitySearch_Chain_HonoursProfessionalBlock(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)
	seedWorkingHour(t, db, 1, int(mpDay.Weekday()), "08:00", "13:00")

	// Pro 2's block covers its OWN chained segment for a candidate start of
	// 10:00 ([10:30, 11:00)), so that start must disappear — while the following
	// one (segment [11:00, 11:30)) stays.
	seedTimeBlockRow(t, db, 1, uintPtr(2), mpDay, "10:30", "11:00")

	chainQuery := "service_ids=10,11&assignments=10:1,11:2&from=" + bookingTestDateKey + "&to=" + bookingTestDateKey + "&center_id=1&limit=50"
	times := searchStartTimes(getSearch(t, r, chainQuery))

	assert.NotContains(t, times, "10:00", "pro 2 is blocked during its chained segment")
	assert.Contains(t, times, "10:30", "the next start falls after the block")

	// The write path agrees: the blocked start is a conflict.
	client := roleToken(t, 1, 0, "client")
	blocked := do(r, http.MethodPost, "/api/reservation/", client,
		assignmentBody([2]uint{10, 11}, [2]uint{1, 2}, "10:00"))
	assert.Equal(t, http.StatusConflict, blocked.Code,
		"a start the chain hides must be rejected: %s", blocked.Body.String())
}

func TestAvailabilitySearch_Chain_HonoursCentreWideBlock(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)
	seedWorkingHour(t, db, 1, int(mpDay.Weekday()), "08:00", "13:00")

	// A centre-wide block applies to BOTH professionals, so any chain whose
	// segments touch it is impossible.
	seedTimeBlockRow(t, db, 1, nil, mpDay, "10:30", "11:00")

	chainQuery := "service_ids=10,11&assignments=10:1,11:2&from=" + bookingTestDateKey + "&to=" + bookingTestDateKey + "&center_id=1&limit=50"
	times := searchStartTimes(getSearch(t, r, chainQuery))

	assert.NotContains(t, times, "10:00", "pro 2's segment is inside the centre-wide block")
	assert.Contains(t, times, "11:00", "the chain fits after the block")
}

// TestAvailabilitySearch_Chain_IgnoresForeignProfessionalBlock guards the scope:
// a block on a professional OUTSIDE the chain must not hide the chain.
func TestAvailabilitySearch_Chain_IgnoresForeignProfessionalBlock(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardPersonnel(t, db, 9, 1, true) // never part of the chain
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)
	seedWorkingHour(t, db, 1, int(mpDay.Weekday()), "08:00", "13:00")

	seedTimeBlockRow(t, db, 1, uintPtr(9), mpDay, "10:00", "12:00")

	chainQuery := "service_ids=10,11&assignments=10:1,11:2&from=" + bookingTestDateKey + "&to=" + bookingTestDateKey + "&center_id=1&limit=50"
	times := searchStartTimes(getSearch(t, r, chainQuery))

	require.NotEmpty(t, times)
	assert.Contains(t, times, "10:00", "a professional outside the chain cannot block it")
}
