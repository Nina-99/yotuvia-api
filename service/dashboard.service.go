package service

import (
	"time"

	"github.com/Nina-99/yotuvia-api/data/response"
)

// DashboardService computes the admin dashboard statistics for a beauty center.
type DashboardService interface {
	// GetStats returns the full stats payload for the caller's center. It
	// verifies the center exists and is active (inactive/deleted → 404
	// ErrNotFound), calculates revenue/ticket aggregates over COMPLETED
	// reservations, and truncates leaders/upcoming lists to 3 entries each.
	GetStats(centerID uint) (*response.DashboardStatsResponse, error)
	// GetStatsWithRange returns the full stats payload plus time series for
	// the given date range [from, to). The service computes UTC boundaries,
	// calls repository by-day queries, and zero-fills missing days so every
	// date in the range is present in the series arrays (never null).
	// churnDays drives the at-risk-clients window (spec: default 30, valid
	// 1–365; controller validates before calling).
	GetStatsWithRange(centerID uint, from, to time.Time, churnDays int) (*response.DashboardStatsResponse, error)
}
