package service

import (
	"fmt"
	"time"
)

// MissionPeriodKey resolves the window key for a mission period. It is the ONE
// place that computes keys, so weekly/monthly rollovers stay consistent.
//
//	once    -> "once"
//	weekly  -> ISO week, e.g. "2026-W45" (Monday–Sunday, UTC)
//	monthly -> "2026-11"
//
// Empty or unknown periods fall back to "once": legacy missions never reset.
func MissionPeriodKey(period string, now time.Time) string {
	utc := now.UTC()
	switch period {
	case "weekly":
		year, week := utc.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case "monthly":
		return utc.Format("2006-01")
	default:
		return "once"
	}
}
