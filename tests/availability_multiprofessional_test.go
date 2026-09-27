package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/router"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  Multi-professional fixtures (Phase 2a)
// ──────────────────────────────────────────────

// mpDay is the Monday used by every multi-professional fixture.
var mpDay = time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

// mpLine is one service line of a hand-built multi-professional reservation.
type mpLine struct {
	serviceID   uint
	personnelID uint
	duration    int
}

// seedMultiProfessionalReservation writes a reservation whose lines belong to
// DIFFERENT professionals — a shape no user flow produces yet, so the fixtures
// build it directly. The reservation's derived lead is the first line's
// professional, mirroring the write-path invariant.
func seedMultiProfessionalReservation(t *testing.T, db *gorm.DB, centerID, leadID uint, date time.Time, start, end string, lines []mpLine) *models.Reservation {
	t.Helper()
	res := &models.Reservation{
		BeautyCenterID: centerID,
		ClientID:       1,
		PersonnelID:    leadID,
		Date:           date,
		StartTime:      start,
		EndTime:        end,
		TotalPrice:     100,
		Status:         "CONFIRMED",
	}
	require.NoError(t, db.Create(res).Error)
	for _, l := range lines {
		pid := l.personnelID
		require.NoError(t, db.Create(&models.ReservationService{
			ReservationID: res.ID,
			ServiceID:     l.serviceID,
			ServiceName:   fmt.Sprintf("svc-%d", l.serviceID),
			Price:         50,
			Duration:      l.duration,
			PersonnelID:   &pid,
		}).Error)
	}
	return res
}

// availabilitySlotTimes returns the times that are NOT available, with reason,
// so a test can assert which specific segment blocks a professional.
func perCenterTimes(t *testing.T, r *gin.Engine, path string) map[string]string {
	t.Helper()
	w := do(r, http.MethodGet, path, "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body struct {
		Slots []struct {
			Time      string `json:"time"`
			Available bool   `json:"available"`
			Reason    string `json:"reason"`
		} `json:"slots"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	out := map[string]string{}
	for _, s := range body.Slots {
		if !s.Available {
			out[s.Time] = s.Reason
		}
	}
	return out
}

// ──────────────────────────────────────────────
//  Agreement + per-segment busy set
// ──────────────────────────────────────────────

// TestAvailabilitySearch_MultiProfessionalAgreement extends the existing
// availability agreement suite (same router, same helpers) to a reservation
// whose two lines belong to two professionals: pro 1 works [10:00,10:30) and
// pro 2 works [10:30,11:00) of the shared 10:00–11:00 envelope.
//
// It proves the read fix two ways:
//   - each professional is busy only for their OWN segment (10:00 blocked for
//     pro 1, 10:30 free; symmetric for pro 2), never for the whole envelope;
//   - the cross-center search and the per-center endpoint return EXACTLY the
//     same slots for each professional (the structural agreement guarantee).
func TestAvailabilitySearch_MultiProfessionalAgreement(t *testing.T) {
	r, db := availabilityTestRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)
	seedWorkingHour(t, db, 1, int(mpDay.Weekday()), "08:00", "12:00")
	seedMultiProfessionalReservation(t, db, 1, 1, mpDay, "10:00", "11:00", []mpLine{
		{serviceID: 10, personnelID: 1, duration: 30},
		{serviceID: 11, personnelID: 2, duration: 30},
	})

	const date = "2026-10-05"

	cases := []struct {
		name            string
		personnelID     uint
		wantUnavailable map[string]string // time → reason
	}{
		{
			name:        "pro 1 blocks only their own segment",
			personnelID: 1,
			wantUnavailable: map[string]string{
				"10:00": "booked",
			},
		},
		{
			name:        "pro 2 blocks only their own segment",
			personnelID: 2,
			wantUnavailable: map[string]string{
				"10:30": "booked",
			},
		},
		{
			name:        "whole center is busy across the whole envelope",
			personnelID: 0,
			wantUnavailable: map[string]string{
				"10:00": "booked",
				"10:30": "booked",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Agreement: search == per-center, unchanged structurally.
			query := fmt.Sprintf("service_ids=10&from=%s&to=%s&center_id=1&limit=50&personnel_id=%d", date, date, tc.personnelID)
			env := getSearch(t, r, query)
			perCenter := getPerCenterAvailable(t, r,
				fmt.Sprintf("/api/beauty-centers/1/available-slots?date=%s&duration=30&personnel_id=%d", date, tc.personnelID))
			assert.Equal(t, perCenter, searchStartTimes(env),
				"search must equal per-center for personnel=%d", tc.personnelID)

			// Explicit per-segment busy set.
			unavailable := perCenterTimes(t, r,
				fmt.Sprintf("/api/beauty-centers/1/available-slots?date=%s&duration=30&personnel_id=%d", date, tc.personnelID))
			assert.Equal(t, tc.wantUnavailable, unavailable)
		})
	}
}

// ──────────────────────────────────────────────
//  Write side: overlap is per segment
// ──────────────────────────────────────────────

// multiProfessionalRouter mounts reservation + availability + beauty-center
// routes over one SQLite DB so a booking can be attempted against the same
// fixtures the availability reads use.
func multiProfessionalRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	validate := validator.New()
	r := gin.New()
	api := r.Group("/api")
	router.ReservationRoutes(api, db, validate)
	router.AvailabilityRoutes(api, db, validate)
	router.BeautyCenterRoutes(api, db, validate)
	return r, db
}

// TestBookingOverlap_MultiProfessionalPerSegment is the READ/WRITE agreement
// acceptance test. Against the same multi-professional reservation:
//
//   - a NEW booking for pro 1 overlapping pro 1's segment is REJECTED (409);
//   - a booking for pro 2 at the SAME wall-clock time is ACCEPTED (201),
//     because their segments do not overlap.
//
// Both use the ordinary admin write path, so the booking flow and the
// availability reads provably agree.
func TestBookingOverlap_MultiProfessionalPerSegment(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Multi", "Client")
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)
	seedWorkingHour(t, db, 1, int(mpDay.Weekday()), "08:00", "12:00")
	seedMultiProfessionalReservation(t, db, 1, 1, mpDay, "10:00", "11:00", []mpLine{
		{serviceID: 10, personnelID: 1, duration: 30},
		{serviceID: 11, personnelID: 2, duration: 30},
	})

	admin := roleToken(t, 7, 1, "beauty_center")
	bodyFor := func(personnelID uint) string {
		return fmt.Sprintf(`{"beauty_center_id":1,"client_id":1,"personnel_id":%d,"service_ids":[10],"date":"2026-10-05","start_time":"10:00"}`, personnelID)
	}

	// Pro 1 is working 10:00–10:30 → conflict.
	rejected := postWalkin(r, admin, bodyFor(1))
	assert.Equal(t, http.StatusConflict, rejected.Code, rejected.Body.String())
	assert.Equal(t, "conflict", errBody(t, rejected.Body.String()))

	// Pro 2 is working 10:30–11:00 → 10:00 is free for them, same wall clock.
	accepted := postWalkin(r, admin, bodyFor(2))
	assert.Equal(t, http.StatusCreated, accepted.Code, accepted.Body.String())
}

// TestAgenda_IncludesNonLeadLine proves GET /reservation/me shows a professional
// every reservation that has a line for them, even when they are not the lead.
func TestAgenda_IncludesNonLeadLine(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardPersonnel(t, db, 3, 1, true)
	seedDashboardClient(t, db, 1, "Multi", "Client")
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)
	shared := seedMultiProfessionalReservation(t, db, 1, 1, mpDay, "10:00", "11:00", []mpLine{
		{serviceID: 10, personnelID: 1, duration: 30},
		{serviceID: 11, personnelID: 2, duration: 30},
	})

	idsFor := func(personnelID uint) []uint {
		w := do(r, http.MethodGet, "/api/reservation/me", roleToken(t, personnelID, 1, "personnel"), "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var got []struct {
			ID uint `json:"id"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
		ids := make([]uint, 0, len(got))
		for _, g := range got {
			ids = append(ids, g.ID)
		}
		return ids
	}

	// The lead (pro 1) and the non-lead (pro 2) both see the reservation.
	assert.Contains(t, idsFor(1), shared.ID)
	assert.Contains(t, idsFor(2), shared.ID)
	// A professional with no line does not.
	assert.NotContains(t, idsFor(3), shared.ID)
}
