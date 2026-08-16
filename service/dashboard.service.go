package service

import "github.com/Nina-99/yottu-api/data/response"

// DashboardService computes the admin dashboard statistics for a beauty center.
type DashboardService interface {
	// GetStats returns the full stats payload for the caller's center. It
	// verifies the center exists and is active (inactive/deleted → 404
	// ErrNotFound), calculates revenue/ticket aggregates over COMPLETED
	// reservations, and truncates leaders/upcoming lists to 3 entries each.
	GetStats(centerID uint) (*response.DashboardStatsResponse, error)
}