package service

import (
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/stretchr/testify/assert"
)

// TestComputeAvailabilitySlots_ExactOutcome pins the extracted kernel's
// observable output so the refactor of GetAvailableSlots is provably
// behaviour-preserving at the unit level.
func TestComputeAvailabilitySlots_ExactOutcome(t *testing.T) {
	date := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC) // not "today"
	hours := []models.WorkingHour{{StartTime: "09:00", EndTime: "11:00"}}

	t.Run("booked and closing", func(t *testing.T) {
		res := []models.Reservation{{StartTime: "09:30", EndTime: "10:30"}}
		got := ComputeAvailabilitySlots(date, 60, hours, res, now)
		requireLen(t, got, 4)
		assert.Equal(t, "09:00", got[0].Time)
		assert.False(t, got[0].Available)
		assert.Equal(t, "booked", got[0].Reason)
		assert.Equal(t, "09:30", got[1].Time)
		assert.False(t, got[1].Available)
		assert.Equal(t, "10:00", got[2].Time)
		assert.False(t, got[2].Available)
		assert.Equal(t, "10:30", got[3].Time)
		assert.False(t, got[3].Available)
		assert.Equal(t, "closing", got[3].Reason)
	})

	t.Run("today filters past slots", func(t *testing.T) {
		today := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
		got := ComputeAvailabilitySlots(date, 30, hours, nil, today)
		requireLen(t, got, 4) // 09:00, 09:30, 10:00, 10:30 (each fits 30m before 11:00)
		assert.Equal(t, "past", got[0].Reason)
		assert.Equal(t, "past", got[1].Reason)
		assert.True(t, got[2].Available) // 10:00 == now, not "past"
		assert.True(t, got[3].Available)
	})

	t.Run("no working hours yields empty", func(t *testing.T) {
		got := ComputeAvailabilitySlots(date, 30, nil, nil, now)
		assert.Empty(t, got)
		assert.NotNil(t, got)
	})
}

func TestAvailabilityPeriodBoundaries(t *testing.T) {
	assert.Equal(t, PeriodMorning, AvailabilityPeriodForMinutes(0))
	assert.Equal(t, PeriodMorning, AvailabilityPeriodForMinutes(11*60+59))
	assert.Equal(t, PeriodAfternoon, AvailabilityPeriodForMinutes(12*60))
	assert.Equal(t, PeriodAfternoon, AvailabilityPeriodForMinutes(17*60+59))
	assert.Equal(t, PeriodEvening, AvailabilityPeriodForMinutes(18*60))
	assert.Equal(t, PeriodEvening, AvailabilityPeriodForMinutes(23*60+59))

	assert.True(t, ValidAvailabilityPeriod(""))
	assert.True(t, ValidAvailabilityPeriod(PeriodMorning))
	assert.False(t, ValidAvailabilityPeriod("dawn"))
}

func TestTotalServicesDuration(t *testing.T) {
	assert.Equal(t, 45, TotalServicesDuration([]models.Service{{Duration: 15}, {Duration: 30}}))
	assert.Equal(t, 0, TotalServicesDuration(nil))
}

func requireLen[T any](t *testing.T, s []T, n int) {
	t.Helper()
	if len(s) != n {
		t.Fatalf("expected %d slots, got %d: %+v", n, len(s), s)
	}
}
