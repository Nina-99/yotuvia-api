package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/data/response"
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
//  Client booking vs availability: one rule, two paths
// ──────────────────────────────────────────────
//
// The read path (GET /beauty-centers/:id/available-slots) only offers slots
// fully inside a working block and hides past slots for today. The write path
// (POST /reservation/) must enforce the SAME rules, or a client can book a slot
// the UI never offered. These tests pin that agreement across both endpoints.

// bookingAvailabilityRouter mounts the availability read endpoint AND the
// reservation write endpoint over the same SQLite DB.
func bookingAvailabilityRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	validate := validator.New()
	r := gin.New()
	api := r.Group("/api")
	router.BeautyCenterRoutes(api, db, validate)
	router.ReservationRoutes(api, db, validate)
	return r, db
}

// fixtureServiceDuration is the total booking duration the fixtures use: it is
// deliberately NOT a multiple of the 30-minute slot step, so the availability
// kernel can emit a slot that no longer fits inside its working block.
const fixtureServiceDuration = 45

// bookingFixture seeds one center, one staff, one client and one 45-minute
// service. 2026-10-05 is a Monday (weekday 1).
func bookingFixture(t *testing.T) (*gin.Engine, *gorm.DB, time.Time) {
	t.Helper()
	r, db := bookingAvailabilityRouter(t)
	seedDashboardCenter(t, db, 1, "Agree Center", "agree@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardClient(t, db, 1, "Book", "Client")
	seedServiceRow(t, db, 1, 1, fixtureServiceDuration)
	return r, db, time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
}

func readSlots(t *testing.T, r *gin.Engine, date string) response.AvailableSlotsResponse {
	t.Helper()
	w := do(r, http.MethodGet,
		fmt.Sprintf("/api/beauty-centers/1/available-slots?date=%s&duration=%d", date, fixtureServiceDuration), "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body response.AvailableSlotsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func postClientBooking(r *gin.Engine, token, date, start string) *httptest.ResponseRecorder {
	body := fmt.Sprintf(
		`{"beauty_center_id":1,"personnel_id":1,"service_ids":[1],"date":%q,"start_time":%q}`,
		date, start)
	return do(r, http.MethodPost, "/api/reservation/", token, body)
}

// TestBookingAvailability_ReadWriteAgree is THE acceptance test: for a given
// center/date every slot the availability endpoint marks available must be
// accepted by the create endpoint, and every slot it marks unavailable for a
// schedule reason (closing) must be rejected. Before the write-side validation
// the create endpoint accepted the closing slot too, so the two disagreed.
func TestBookingAvailability_ReadWriteAgree(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	// Snapshot the slot list once from a read-only fixture. A close time that is
	// not a multiple of the slot step makes the kernel emit a slot that no
	// longer fits the 45-minute booking ("closing").
	readRouter, readDB, date := bookingFixture(t)
	seedWorkingHour(t, readDB, 1, int(date.Weekday()), "09:00", "18:10")
	day := date.Format("2006-01-02")
	slots := readSlots(t, readRouter, day)
	require.NotEmpty(t, slots.Slots)

	token := roleToken(t, 1, 0, "client")
	sawClosing := false
	for _, slot := range slots.Slots {
		slot := slot
		t.Run(slot.Time, func(t *testing.T) {
			// Book against a FRESH DB so an earlier booking cannot make a later
			// slot conflict: each slot is judged independently.
			writeRouter, writeDB, wdate := bookingFixture(t)
			seedWorkingHour(t, writeDB, 1, int(wdate.Weekday()), "09:00", "18:10")

			w := postClientBooking(writeRouter, token, day, slot.Time)
			if slot.Available {
				assert.Equal(t, http.StatusCreated, w.Code,
					"a slot the read path offers must be bookable: %s", w.Body.String())
				return
			}
			require.Equal(t, "closing", slot.Reason, "fixture only expects closing reasons here")
			sawClosing = true
			assert.Equal(t, http.StatusBadRequest, w.Code,
				"a slot that does not fit the working block must be rejected: %s", w.Body.String())
		})
	}
	assert.True(t, sawClosing, "the fixture must exercise a slot outside the working block")
}

// TestBookingAvailability_PastSlotToday ties the write rejection to the read
// list for TODAY: whichever past slot the availability endpoint hides for being
// in the past must be rejected by the create endpoint.
func TestBookingAvailability_PastSlotToday(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db, _ := bookingFixture(t)

	now := time.Now()
	today := now.Format("2006-01-02")
	seedWorkingHour(t, db, 1, int(now.Weekday()), "00:00", "23:59")

	slots := readSlots(t, r, today).Slots
	var past *response.AvailableSlot
	for i := range slots {
		if !slots[i].Available && slots[i].Reason == "past" {
			past = &slots[i]
			break
		}
	}
	if past == nil {
		t.Skip("no past slot yet today (test started right after midnight)")
	}

	w := postClientBooking(r, roleToken(t, 1, 0, "client"), today, past.Time)
	assert.Equal(t, http.StatusBadRequest, w.Code,
		"a past slot the read path hides must be rejected: %s", w.Body.String())
}

// TestClientBooking_WorkingHoursEnforced covers each schedule rule on the
// client create endpoint. Every case is invalid input, so the status is 400
// (not 409, which stays reserved for an overlap on an offered slot).
func TestClientBooking_WorkingHoursEnforced(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	tests := []struct {
		name     string
		hours    [][2]string
		otherDay bool // seed the block on a different weekday, not the booking date
		start    string
		want     int
	}{
		{
			name:  "before opening",
			hours: [][2]string{{"09:00", "18:00"}},
			start: "08:30",
			want:  http.StatusBadRequest,
		},
		{
			name:  "after closing",
			hours: [][2]string{{"09:00", "18:00"}},
			start: "17:45", // +30min ends 18:15, past the block
			want:  http.StatusBadRequest,
		},
		{
			name:  "spans two working blocks",
			hours: [][2]string{{"09:00", "12:00"}, {"13:00", "18:00"}},
			start: "11:45", // +30min ends 12:15, inside neither block
			want:  http.StatusBadRequest,
		},
		{
			name:     "closed weekday has no block",
			otherDay: true,
			start:    "10:00",
			want:     http.StatusBadRequest,
		},
		{
			name:  "no working hours configured at all",
			hours: nil,
			start: "10:00",
			want:  http.StatusBadRequest,
		},
		{
			name:  "valid in-hours booking succeeds",
			hours: [][2]string{{"09:00", "18:00"}},
			start: "10:00",
			want:  http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, db, date := bookingFixture(t)
			for _, h := range tt.hours {
				seedWorkingHour(t, db, 1, int(date.Weekday()), h[0], h[1])
			}
			if tt.otherDay {
				other := (int(date.Weekday()) + 1) % 7
				seedWorkingHour(t, db, 1, other, "09:00", "18:00")
			}

			w := postClientBooking(r, roleToken(t, 1, 0, "client"), date.Format("2006-01-02"), tt.start)
			assert.Equal(t, tt.want, w.Code, "body: %s", w.Body.String())
		})
	}
}

// TestClientBooking_AdminWalkinExempt proves the deliberate asymmetry: the
// admin/walk-in path takes responsibility for its manual bookings and may book
// outside posted hours. The client path, by contrast, may not (see above).
func TestClientBooking_AdminWalkinExempt(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	r, db, date := bookingFixture(t)
	seedDashboardClient(t, db, 4, "Walk", "In")
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")

	adminToken := roleToken(t, 7, 1, "beauty_center")
	for _, start := range []string{"07:00", "20:00"} {
		t.Run(start, func(t *testing.T) {
			body := fmt.Sprintf(
				`{"beauty_center_id":1,"client_id":4,"personnel_id":1,"service_ids":[1],"date":%q,"start_time":%q}`,
				date.Format("2006-01-02"), start)
			w := do(r, http.MethodPost, "/api/reservation/admin", adminToken, body)
			assert.Equal(t, http.StatusCreated, w.Code,
				"walk-ins may be booked outside posted hours: %s", w.Body.String())
		})
	}
}
