package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  Time blocks vs availability: read and write agree
// ──────────────────────────────────────────────
//
// A block (break/holiday) must subtract from availability exactly like a busy
// reservation: the read path (GET .../available-slots) hides the blocked window
// AND the write path rejects a booking inside it. These tests pin that
// agreement, mirroring TestBookingAvailability_ReadWriteAgree for blocks.

// seedTimeBlockRow inserts a block directly (bypassing the JWT-scoped endpoint)
// so the availability/booking paths can be exercised in isolation.
func seedTimeBlockRow(t *testing.T, db *gorm.DB, centerID uint, personnelID *uint, date time.Time, start, end string) {
	t.Helper()
	block := models.TimeBlock{
		BeautyCenterID: centerID,
		PersonnelID:    personnelID,
		Date:           date,
		StartTime:      start,
		EndTime:        end,
		Reason:         "break",
		CreatedByRole:  "beauty_center",
	}
	require.NoError(t, db.Create(&block).Error)
}

// slotAvailable reports whether the slot list offers the given start time.
func slotAvailable(slots []response.AvailableSlot, start string) bool {
	for _, s := range slots {
		if s.Time == start {
			return s.Available
		}
	}
	return false
}

// readSlotsFor reads the per-center availability list, optionally narrowed to a
// professional preference (0 = any professional).
func readSlotsFor(t *testing.T, r *gin.Engine, date string, personnelID uint) response.AvailableSlotsResponse {
	t.Helper()
	url := fmt.Sprintf("/api/beauty-centers/1/available-slots?date=%s&duration=%d", date, fixtureServiceDuration)
	if personnelID > 0 {
		url += fmt.Sprintf("&personnel_id=%d", personnelID)
	}
	w := do(r, http.MethodGet, url, "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body response.AvailableSlotsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

// TestTimeBlock_RemovesSlotFromAvailability proves a centre-wide block hides the
// blocked window from the read path while leaving the rest of the day intact.
func TestTimeBlock_RemovesSlotFromAvailability(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	for _, tt := range []struct {
		name        string
		personnelID *uint
		queryBy     uint
	}{
		{name: "centre-wide", personnelID: nil, queryBy: 0},
		{name: "professional", personnelID: uintPtr(1), queryBy: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, db, date := bookingFixture(t)
			seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")

			day := date.Format("2006-01-02")
			before := readSlotsFor(t, r, day, tt.queryBy)
			require.True(t, slotAvailable(before.Slots, "10:00"), "10:00 must be offered before the block")

			seedTimeBlockRow(t, db, 1, tt.personnelID, date, "10:00", "11:00")

			after := readSlotsFor(t, r, day, tt.queryBy)
			assert.False(t, slotAvailable(after.Slots, "10:00"),
				"a blocked window must not be offered")
			assert.True(t, slotAvailable(after.Slots, "14:00"),
				"slots outside the block must still be offered")
		})
	}
}

// TestTimeBlock_ForeignProfessionalDoesNotHideCentre is the scoping guard: one
// professional's break must not hide the centre for the others.
func TestTimeBlock_ForeignProfessionalDoesNotHideCentre(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	r, db, date := bookingFixture(t)
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")
	// A block on personnel 2 (another professional). Booking asks for personnel 1.
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedTimeBlockRow(t, db, 1, uintPtr(2), date, "10:00", "11:00")

	slots := readSlots(t, r, date.Format("2006-01-02"))
	assert.True(t, slotAvailable(slots.Slots, "10:00"),
		"another professional's block must not hide personnel 1's slot")
}

// TestTimeBlock_RejectsBooking is the write-side counterpart: a window the read
// path hides must not be bookable (409), while an unblocked slot still books.
func TestTimeBlock_RejectsBooking(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	r, db, date := bookingFixture(t)
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")
	seedTimeBlockRow(t, db, 1, nil, date, "10:00", "11:00")

	token := roleToken(t, 1, 0, "client")
	day := date.Format("2006-01-02")

	w := postClientBooking(r, token, day, "10:00")
	assert.Equal(t, http.StatusConflict, w.Code,
		"a booking inside a block must be rejected as a conflict: %s", w.Body.String())

	w = postClientBooking(r, token, day, "14:00")
	assert.Equal(t, http.StatusCreated, w.Code,
		"an unblocked in-hours slot must still be bookable: %s", w.Body.String())
}
