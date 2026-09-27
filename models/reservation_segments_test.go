package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func linePtr(id, duration uint, personnel *uint) ReservationService {
	return ReservationService{ID: id, Duration: int(duration), PersonnelID: personnel}
}

func ptrPersonnel(id uint) *uint { return &id }

// TestDeriveReservationSegments_SingleProfessionalIsNoOp is THE safety property
// of the per-segment refactor: for a single-professional reservation the union
// of the derived segments must equal the stored envelope EXACTLY. If this
// holds, every existing reservation reads and writes the same as before.
func TestDeriveReservationSegments_SingleProfessionalIsNoOp(t *testing.T) {
	pro := uint(7)
	res := &Reservation{
		PersonnelID: pro,
		StartTime:   "10:00",
		EndTime:     "11:30", // 10:00 + (30 + 60)
		ReservationServices: []ReservationService{
			linePtr(2, 60, ptrPersonnel(pro)),
			linePtr(1, 30, ptrPersonnel(pro)),
		},
	}

	segments := DeriveReservationSegments(res)
	require.Len(t, segments, 2)

	// Lines are consumed in id order (1 then 2), regardless of slice order.
	assert.Equal(t, Segment{PersonnelID: pro, StartMinutes: 600, EndMinutes: 630}, segments[0])
	assert.Equal(t, Segment{PersonnelID: pro, StartMinutes: 630, EndMinutes: 690}, segments[1])

	// Union == envelope, byte for byte (no gap, no overlap).
	assert.Equal(t, ParseClockMinutes(res.StartTime), segments[0].StartMinutes)
	assert.Equal(t, ParseClockMinutes(res.EndTime), segments[len(segments)-1].EndMinutes)
	for i := 1; i < len(segments); i++ {
		assert.Equal(t, segments[i-1].EndMinutes, segments[i].StartMinutes, "segments must tile contiguously")
	}
}

// TestDeriveReservationSegments_MultiProfessional pins the sequential split
// when lines belong to different professionals.
func TestDeriveReservationSegments_MultiProfessional(t *testing.T) {
	res := &Reservation{
		PersonnelID: 1, // derived lead = first line's professional
		StartTime:   "10:00",
		EndTime:     "11:00", // 10:00 + (30 + 30)
		ReservationServices: []ReservationService{
			linePtr(1, 30, ptrPersonnel(1)),
			linePtr(2, 30, ptrPersonnel(2)),
		},
	}

	segments := DeriveReservationSegments(res)
	require.Len(t, segments, 2)
	assert.Equal(t, Segment{PersonnelID: 1, StartMinutes: 600, EndMinutes: 630}, segments[0])
	assert.Equal(t, Segment{PersonnelID: 2, StartMinutes: 630, EndMinutes: 660}, segments[1])
}

// TestDeriveReservationSegments_Fallbacks covers the legacy shapes: a nil line
// professional inherits the lead, and a line-less reservation keeps its
// envelope.
func TestDeriveReservationSegments_Fallbacks(t *testing.T) {
	t.Run("nil line professional inherits lead", func(t *testing.T) {
		res := &Reservation{
			PersonnelID: 9,
			StartTime:   "09:00",
			EndTime:     "09:30",
			ReservationServices: []ReservationService{
				linePtr(1, 30, nil),
			},
		}
		segments := DeriveReservationSegments(res)
		require.Len(t, segments, 1)
		assert.Equal(t, uint(9), segments[0].PersonnelID)
	})

	t.Run("no lines falls back to envelope", func(t *testing.T) {
		res := &Reservation{PersonnelID: 3, StartTime: "14:00", EndTime: "15:00"}
		segments := DeriveReservationSegments(res)
		require.Len(t, segments, 1)
		assert.Equal(t, Segment{PersonnelID: 3, StartMinutes: 840, EndMinutes: 900}, segments[0])
	})
}

// TestReservationBusyForPersonnel_ExcludesOtherSegments is the overlap
// predicate's core: a professional is busy only for THEIR segments.
func TestReservationBusyForPersonnel_ExcludesOtherSegments(t *testing.T) {
	res := &Reservation{
		PersonnelID: 1,
		StartTime:   "10:00",
		EndTime:     "11:00",
		ReservationServices: []ReservationService{
			linePtr(1, 30, ptrPersonnel(1)),
			linePtr(2, 30, ptrPersonnel(2)),
		},
	}

	// Pro 1 is busy at 10:00–10:30, free at 10:30–11:00.
	assert.True(t, ReservationBusyForPersonnel(res, 1, 600, 630))
	assert.False(t, ReservationBusyForPersonnel(res, 1, 630, 660))
	// Symmetric for pro 2.
	assert.False(t, ReservationBusyForPersonnel(res, 2, 600, 630))
	assert.True(t, ReservationBusyForPersonnel(res, 2, 630, 660))
	// A professional with no line is never busy.
	assert.False(t, ReservationBusyForPersonnel(res, 99, 600, 660))
}
