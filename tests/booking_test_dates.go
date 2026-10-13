package tests

import "time"

// ─────────────────────────────────────────────────────────────
//  Deterministic FUTURE date anchor for booking/availability tests.
//
//  These suites book/reserve on a concrete day and seed working hours for that
//  day's weekday. The old hard-coded 2026-10-05 only worked while the wall
//  clock was before that date; once the day passed, every "available slot"
//  assertion went empty (past day), flaking CI at night/the next day.
//
//  Anchoring to TOMORROW (UTC midnight) keeps the window in the future no
//  matter when the suite runs, while remaining deterministic within a run.
// ─────────────────────────────────────────────────────────────

// bookingTestDate is a deterministic FUTURE anchor (today + 5 days at UTC
// midnight) — the canonical day used by the availability/booking suites. The
// +5 offset keeps the original tests' relative offsets (base-4 … base+11) in
// the future too, so their span/validation semantics are preserved.
var bookingTestDate = time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, 5)

// bookingTestDateKey is bookingTestDate formatted as YYYY-MM-DD for query
// strings and JSON bodies.
var bookingTestDateKey = bookingTestDate.Format("2006-01-02")

// bookingTestDatePlus returns bookingTestDate shifted by days, formatted as
// YYYY-MM-DD (negative shifts go backwards).
func bookingTestDatePlus(days int) string {
	return bookingTestDate.AddDate(0, 0, days).Format("2006-01-02")
}
