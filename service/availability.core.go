package service

// This file holds the single, shared slot-generation core used by BOTH the
// per-center endpoint (GET /beauty-centers/:id/available-slots) and the
// cross-center search (GET /availability/search).
//
// It is deliberately pure: no database, no clock, no HTTP. Callers load the
// working hours and reservations however they like (one center or a batch) and
// hand them in; the same ComputeAvailabilitySlots call then produces the same
// slots. That is what makes "search == per-center" a structural guarantee
// instead of a promise two implementations might quietly drift away from.
//
// It lives in the service package (not a dedicated one) on purpose: a separate
// package would push `go list ./...` past the project's 15-package invariant.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
)

// SlotStepMinutes is the fixed candidate granularity: slots are offered every
// 30 minutes from each working block's start.
const SlotStepMinutes = 30

// Period identifiers accepted by the search `period` filter.
const (
	PeriodMorning   = "morning"
	PeriodAfternoon = "afternoon"
	PeriodEvening   = "evening"
)

// Period boundaries, documented explicitly:
//
//	morning   → [00:00, 12:00)
//	afternoon → [12:00, 18:00)
//	evening   → [18:00, 24:00)
const (
	periodAfternoonStart = 12 * 60
	periodEveningStart   = 18 * 60
)

// ValidAvailabilityPeriod reports whether p is a recognised period filter. The
// empty string means "no period filter" and is also valid.
func ValidAvailabilityPeriod(p string) bool {
	switch p {
	case "", PeriodMorning, PeriodAfternoon, PeriodEvening:
		return true
	default:
		return false
	}
}

// AvailabilityPeriodForMinutes maps a time-of-day (minutes since midnight) to
// its period.
func AvailabilityPeriodForMinutes(minutes int) string {
	switch {
	case minutes < periodAfternoonStart:
		return PeriodMorning
	case minutes < periodEveningStart:
		return PeriodAfternoon
	default:
		return PeriodEvening
	}
}

// ParseScheduleMinutes converts "HH:MM" to minutes since midnight. Malformed
// input yields the zero value, matching the historical behaviour of the service
// helper it replaces.
func ParseScheduleMinutes(timeStr string) int {
	parts := strings.Split(timeStr, ":")
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	return h*60 + m
}

// TotalServicesDuration sums service durations exactly the way the booking flow
// does (createReservationCore / Update). Kept here so both the write path and
// the availability search share one definition instead of two that could drift.
func TotalServicesDuration(services []models.Service) int {
	total := 0
	for _, svc := range services {
		total += svc.Duration
	}
	return total
}

// ComputeAvailabilitySlots is the extracted availability kernel. Given a date,
// the total booking duration, a center's working blocks for that weekday and
// that date's reservations, it returns the FULL slot list (available and
// unavailable with reasons), replicating the per-center endpoint byte for byte.
//
//	now is injected so callers control "today" filtering deterministically.
//
// Working blocks are sorted by start time so the output does not depend on the
// order the caller happened to load them in; the batched and per-center reads
// therefore agree.
func ComputeAvailabilitySlots(
	date time.Time,
	durationMinutes int,
	schedules []models.WorkingHour,
	reservations []models.Reservation,
	now time.Time,
) []response.AvailableSlot {
	if len(schedules) == 0 {
		return []response.AvailableSlot{}
	}

	blocks := make([]models.WorkingHour, len(schedules))
	copy(blocks, schedules)
	sort.SliceStable(blocks, func(i, j int) bool {
		return ParseScheduleMinutes(blocks[i].StartTime) < ParseScheduleMinutes(blocks[j].StartTime)
	})

	isBooked := func(slotStart, slotEnd int) bool {
		for _, r := range reservations {
			rStart := ParseScheduleMinutes(r.StartTime)
			rEnd := ParseScheduleMinutes(r.EndTime)
			if slotStart < rEnd && slotEnd > rStart {
				return true
			}
		}
		return false
	}

	nowMinutes := now.Hour()*60 + now.Minute()
	isToday := date.Year() == now.Year() && date.YearDay() == now.YearDay()

	var slots []response.AvailableSlot
	for _, schedule := range blocks {
		startMinutes := ParseScheduleMinutes(schedule.StartTime)
		endMinutes := ParseScheduleMinutes(schedule.EndTime)

		for m := startMinutes; m+SlotStepMinutes <= endMinutes; m += SlotStepMinutes {
			timeStr := fmt.Sprintf("%02d:%02d", m/60, m%60)

			// Slot is only feasible if the full duration fits within this block.
			slotEnd := m + durationMinutes
			if slotEnd > endMinutes {
				slots = append(slots, response.AvailableSlot{
					Time:      timeStr,
					Available: false,
					Reason:    "closing",
				})
				continue
			}

			available := !isBooked(m, slotEnd)
			if isToday && m < nowMinutes {
				available = false
			}
			reason := ""
			if !available && isBooked(m, m+durationMinutes) {
				reason = "booked"
			} else if !available && isToday && m < nowMinutes {
				reason = "past"
			}
			slots = append(slots, response.AvailableSlot{
				Time:      timeStr,
				Available: available,
				Reason:    reason,
			})
		}
	}

	return slots
}
