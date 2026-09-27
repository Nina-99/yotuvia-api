package service

import "github.com/Nina-99/yottu-api/models"

// This file holds the multi-professional CHAIN evaluation used by the
// availability search. It deliberately does NOT introduce a second interval
// engine: it reuses the Phase 2a machinery —
//
//   - PersonnelBusyReservations projects a center's reservations down to ONE
//     professional's own busy intervals (the same projection the single-
//     professional search already uses);
//   - models.IntervalsOverlap is the ONE half-open overlap predicate from the
//     shared segment kernel.
//
// The only new thing is the ORDERED walk: for a candidate start t, professional
// k must be free for their segment [t + Σ previous durations, + own duration).
//
// It lives in the service package (not a dedicated one) on purpose: a separate
// package would push `go list ./...` past the project's 15-package invariant.

// ChainStep is one ordered step of a multi-professional plan: WHO works and for
// HOW LONG. The position in the slice defines the offset from the candidate
// start (Σ durations of the previous steps).
type ChainStep struct {
	PersonnelID uint
	Duration    int
}

// ChainPersonnel returns the DISTINCT professionals in the chain, in first-seen
// order, so a reservation load (or any batch) touches each professional once.
func ChainPersonnel(chain []ChainStep) []uint {
	seen := make(map[uint]struct{}, len(chain))
	out := make([]uint, 0, len(chain))
	for _, step := range chain {
		if _, ok := seen[step.PersonnelID]; ok {
			continue
		}
		seen[step.PersonnelID] = struct{}{}
		out = append(out, step.PersonnelID)
	}
	return out
}

// ChainBusyByPersonnel projects the date's reservations down to each DISTINCT
// chain professional's busy minute intervals. Each professional is projected
// ONCE (never once per slot) and the result is a pure in-memory map, so the
// chain evaluation adds no per-professional or per-slot query.
func ChainBusyByPersonnel(reservations []models.Reservation, chain []ChainStep) map[uint][][2]int {
	busy := make(map[uint][][2]int, len(chain))
	for _, step := range chain {
		if _, done := busy[step.PersonnelID]; done {
			continue
		}
		projected := PersonnelBusyReservations(reservations, step.PersonnelID)
		intervals := make([][2]int, 0, len(projected))
		for i := range projected {
			intervals = append(intervals, [2]int{
				ParseScheduleMinutes(projected[i].StartTime),
				ParseScheduleMinutes(projected[i].EndTime),
			})
		}
		busy[step.PersonnelID] = intervals
	}
	return busy
}

// ChainFits reports whether the whole chain fits at startMinutes: for every
// step, the professional must be free across
// [start + Σ previous durations, + own duration).
//
// It reuses models.IntervalsOverlap — the same predicate the write side's
// per-segment overlap check uses — so a slot the search offers and a booking it
// accepts can never disagree on an interval boundary.
func ChainFits(startMinutes int, chain []ChainStep, busy map[uint][][2]int) bool {
	cursor := startMinutes
	for _, step := range chain {
		segEnd := cursor + step.Duration
		for _, iv := range busy[step.PersonnelID] {
			if models.IntervalsOverlap(cursor, segEnd, iv[0], iv[1]) {
				return false
			}
		}
		cursor = segEnd
	}
	return true
}
