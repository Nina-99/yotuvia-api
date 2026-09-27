package models

import (
	"sort"
	"strconv"
	"strings"
)

// This file holds the single, shared kernel that turns a reservation and its
// ordered service lines into the per-professional busy intervals it actually
// occupies. It is the ONE definition of "who is busy when" used by both the
// WRITE side (repository.CheckOverlap, via the create/update/assign paths) and
// the READ side (availability slots and the personnel agenda).
//
// It lives in the models package — an EXISTING package, so the project's
// `go list ./...` invariant (15 packages) is untouched — rather than next to
// the availability kernel in `service`, because the repository layer must call
// it too and `repository` cannot import `service` (service imports repository;
// a reverse import would be a cycle). `models` is the lowest layer both share,
// so the pure kernel sits here and the read/write paths consume the same
// function instead of two implementations that could drift.

// Segment is a contiguous interval of ONE professional's time inside a
// reservation, measured in minutes since midnight on the reservation's date.
type Segment struct {
	PersonnelID  uint
	StartMinutes int
	EndMinutes   int
}

// ParseClockMinutes converts "HH:MM" to minutes since midnight. Malformed input
// yields the zero value. This is the single definition of the parse used by the
// availability kernel (service.ParseScheduleMinutes delegates here) and by the
// overlap check, so the read and write paths can never disagree on a boundary.
func ParseClockMinutes(timeStr string) int {
	parts := strings.Split(timeStr, ":")
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	return h*60 + m
}

// DeriveReservationSegments turns a reservation and its ordered service lines
// into the segments it occupies, split per professional.
//
// DERIVATION RULE — sequential, in line order (ascending id = creation order):
//
//	line i occupies [start + Σ d(0..i-1), start + Σ d(0..i))
//
// where d(k) is line k's duration. This is exactly the semantics the booking
// flow already persists: createReservationCore and Update both set
// EndTime = StartTime + Σ durations. For a SINGLE-PROFESSIONAL reservation the
// segments tile [StartTime, EndTime) contiguously with no gap and no overlap,
// so their union is the stored envelope byte for byte — the property that makes
// per-segment reading and writing a behavioural no-op for every existing row.
//
// A line whose PersonnelID is nil (pre-attribution legacy rows) inherits the
// reservation's derived lead. A reservation with no service lines at all falls
// back to one envelope segment for the lead, so rows written before service
// lines existed keep blocking exactly as before.
func DeriveReservationSegments(r *Reservation) []Segment {
	startMinutes := ParseClockMinutes(r.StartTime)
	endMinutes := ParseClockMinutes(r.EndTime)

	lines := make([]ReservationService, len(r.ReservationServices))
	copy(lines, r.ReservationServices)
	if len(lines) == 0 {
		return []Segment{{
			PersonnelID:  r.PersonnelID,
			StartMinutes: startMinutes,
			EndMinutes:   endMinutes,
		}}
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].ID < lines[j].ID })

	segments := make([]Segment, 0, len(lines))
	cursor := startMinutes
	for _, line := range lines {
		personnel := r.PersonnelID
		if line.PersonnelID != nil {
			personnel = *line.PersonnelID
		}
		next := cursor + line.Duration
		segments = append(segments, Segment{
			PersonnelID:  personnel,
			StartMinutes: cursor,
			EndMinutes:   next,
		})
		cursor = next
	}
	return segments
}

// IntervalsOverlap reports whether the half-open windows [aStart,aEnd) and
// [bStart,bEnd) intersect.
func IntervalsOverlap(aStart, aEnd, bStart, bEnd int) bool {
	return aStart < bEnd && aEnd > bStart
}

// ReservationBusyForPersonnel reports whether the reservation occupies any time
// for personnelID inside [startMinutes, endMinutes), evaluated over the
// professional's own derived segments — never over the whole envelope.
func ReservationBusyForPersonnel(r *Reservation, personnelID uint, startMinutes, endMinutes int) bool {
	for _, seg := range DeriveReservationSegments(r) {
		if seg.PersonnelID != personnelID {
			continue
		}
		if IntervalsOverlap(seg.StartMinutes, seg.EndMinutes, startMinutes, endMinutes) {
			return true
		}
	}
	return false
}
