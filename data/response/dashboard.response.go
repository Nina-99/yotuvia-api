package response

import "time"

// TopServiceDTO is one rank in the top-services leaderboard. Revenue is the
// SUM of reservation-time prices over COMPLETED reservations in range
// (feat-admin-analytics D5); appointment_count remains populated during the
// transition from the legacy count-only ranking.
type TopServiceDTO struct {
	ServiceID        uint    `json:"service_id"`
	ServiceName      string  `json:"service_name"`
	AppointmentCount int     `json:"appointment_count"`
	Revenue          float64 `json:"revenue"`
}

// PersonnelRevenueDTO is one rank of the staff revenue leaderboard.
type PersonnelRevenueDTO struct {
	PersonnelID      uint    `json:"personnel_id"`
	PersonnelName    string  `json:"personnel_name"`
	Revenue          float64 `json:"revenue"`
	AppointmentCount int     `json:"appointment_count"`
}

// ClientSpendDTO is one rank of the top-clients-by-spend leaderboard.
type ClientSpendDTO struct {
	ClientID   uint    `json:"client_id"`
	ClientName string  `json:"client_name"`
	TotalSpent float64 `json:"total_spent"`
	VisitCount int     `json:"visit_count"`
}

// AtRiskClientDTO is a client whose last COMPLETED visit predates the churn
// window; days_since counts full days between last visit and today (UTC).
type AtRiskClientDTO struct {
	ClientID      uint   `json:"client_id"`
	ClientName    string `json:"client_name"`
	LastVisitDate string `json:"last_visit_date"` // "2006-01-02"
	DaysSince     int    `json:"days_since"`
}

// PersonnelOccupancyDTO compares a staff member's booked minutes against the
// center-level schedule availability in range. The schema has no per-person
// schedules, so available_minutes is the shared center denominator — values
// are valid for RELATIVE comparison across staff (design caveat, feat-admin-
// analytics D7 revision).
type PersonnelOccupancyDTO struct {
	PersonnelID      uint    `json:"personnel_id"`
	PersonnelName    string  `json:"personnel_name"`
	BookedMinutes    int     `json:"booked_minutes"`
	AvailableMinutes int     `json:"available_minutes"`
	OccupancyPct     float64 `json:"occupancy_pct"` // one decimal; may exceed 100 on overlaps
}

// UpcomingReservationDTO is the lightweight reservation slice rendered in the
// admin dashboard "next bookings" list.
type UpcomingReservationDTO struct {
	ID         uint    `json:"id"`
	ClientName string  `json:"client_name"`
	Date       string  `json:"date"`
	StartTime  string  `json:"start_time"`
	EndTime    string  `json:"end_time"`
	Status     string  `json:"status"`
	TotalPrice float64 `json:"total_price"`
}

// SeriesDayDTO represents a single day's value in a time series.
type SeriesDayDTO struct {
	Date  string  `json:"date"` // "2006-01-02"
	Value float64 `json:"value"`
}

// DashboardStatsResponse is the single server-computed stats payload returned
// by GET /dashboard/stats for an authenticated beauty_center actor, scoped to
// the caller's centerID claim.
//
// Phase-2 fields (feat-admin-analytics): deltas compare the requested window
// against the immediately preceding window of equal length — nil when the
// previous window has no baseline (JSON null). hour_heatmap is always a full
// 7×24 matrix (index 0 = Sunday). Arrays are never null (wire contract).
type DashboardStatsResponse struct {
	AppointmentsToday    int                      `json:"appointments_today"`
	PendingAppointments  int                      `json:"pending_appointments"`
	TotalServices        int                      `json:"total_services"`
	TotalPersonnel       int                      `json:"total_personnel"`
	TotalReviews         int                      `json:"total_reviews"`
	AverageRating        float64                  `json:"average_rating"`
	RevenueToday         float64                  `json:"revenue_today"`
	RevenueWeek          float64                  `json:"revenue_week"`
	RevenueMonth         float64                  `json:"revenue_month"`
	AverageTicket        float64                  `json:"average_ticket"`
	TopServices          []TopServiceDTO          `json:"top_services"`
	UpcomingReservations []UpcomingReservationDTO `json:"upcoming_reservations"`
	RevenueSeries        []SeriesDayDTO           `json:"revenue_series"`
	AppointmentsSeries   []SeriesDayDTO           `json:"appointments_series"`

	RevenueDeltaPct      *float64              `json:"revenue_delta_pct"`
	AppointmentsDeltaPct *float64              `json:"appointments_delta_pct"`
	CancelledCount       int                   `json:"cancelled_count"`
	TotalAppointments    int                   `json:"total_appointments"`
	CancellationRatePct  float64               `json:"cancellation_rate_pct"`
	HourHeatmap          [][]int               `json:"hour_heatmap"`
	PersonnelRevenue     []PersonnelRevenueDTO `json:"personnel_revenue"`

	// ── Phase 3 (feat-admin-analytics) — never-null arrays, zero-safe scalars.
	NewClients         int                     `json:"new_clients"`
	RecurringClients   int                     `json:"recurring_clients"`
	TopClients         []ClientSpendDTO        `json:"top_clients"`
	AtRiskClients      []AtRiskClientDTO       `json:"at_risk_clients"`
	PersonnelOccupancy []PersonnelOccupancyDTO `json:"personnel_occupancy"`
	TicketSeries       []SeriesDayDTO          `json:"ticket_series"`
	RatingSeries       []SeriesDayDTO          `json:"rating_series"`

	// ── Token / Ytv balance (feat-token-economy) ──
	Balance          float64 `json:"balance"`
	OverdraftLimit   float64 `json:"overdraft_limit"`
	AvailableBalance float64 `json:"available_balance"`

	// TokenHealth tells the owner WHY the wallet matters: whether bookings are
	// still accepted, and for how long it has been unable to fund one. The
	// dashboard banner reads this; the booking gate uses the same source.
	TokenHealth *TokenHealthDTO `json:"token_health,omitempty"`
}

// TokenHealthDTO mirrors service.TokenHealth on the wire.
type TokenHealthDTO struct {
	// State: "ok" | "low" | "deficit" | "suspended".
	State             string     `json:"state"`
	DeficitSince      *time.Time `json:"deficit_since,omitempty"`
	DeficitDays       int        `json:"deficit_days"`
	CanAcceptBookings bool       `json:"can_accept_bookings"`
}
