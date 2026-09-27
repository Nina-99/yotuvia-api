package repository

import (
	"time"

	"github.com/Nina-99/yottu-api/models"
)

// TopServiceRow is the aggregated leaderboard row returned by TopServices: a
// service's id/name snapshot plus how many reservations reference it.
type TopServiceRow struct {
	ServiceID        uint    `gorm:"column:service_id"`
	ServiceName      string  `gorm:"column:service_name"`
	AppointmentCount int64   `gorm:"column:appointment_count"`
	Revenue          float64 `gorm:"column:revenue"`
}

// RevenueDayRow is a single day's revenue aggregate returned by RevenueByDay.
type RevenueDayRow struct {
	Date  time.Time
	Value float64
}

// CountDayRow is a single day's appointment count returned by AppointmentsByDay.
type CountDayRow struct {
	Date  time.Time
	Value int64
}

// HeatmapRow is one raw COMPLETED reservation feeding the day×hour heatmap.
// Aggregation into the 7×24 matrix happens in the service layer (design
// decision D4) to stay portable across PostgreSQL and the SQLite test driver.
type HeatmapRow struct {
	Date      time.Time
	StartTime string
}

// CountStatusRow bundles total vs CANCELLED reservation counts for a range,
// feeding the cancellation-rate metric in one round trip.
type CountStatusRow struct {
	Total     int64 `gorm:"column:total"`
	Cancelled int64 `gorm:"column:cancelled"`
}

// PersonnelRevenueRow is one rank of the revenue-by-personnel leaderboard.
type PersonnelRevenueRow struct {
	PersonnelID      uint    `gorm:"column:personnel_id"`
	PersonnelName    string  `gorm:"column:personnel_name"`
	Revenue          float64 `gorm:"column:revenue"`
	AppointmentCount int64   `gorm:"column:appointment_count"`
}

// ClientRecurrenceRow holds the new-vs-recurring client counts (Phase 3).
// "New" = client whose earliest non-CANCELLED reservation starts inside the
// range; "Recurring" = visited in range but first visit predates it.
type ClientRecurrenceRow struct {
	NewClients       int64 `gorm:"column:new_clients"`
	RecurringClients int64 `gorm:"column:recurring_clients"`
}

// ClientSpendRow is one rank of the top-clients-by-spend leaderboard.
type ClientSpendRow struct {
	ClientID   uint    `gorm:"column:client_id"`
	ClientName string  `gorm:"column:client_name"`
	TotalSpent float64 `gorm:"column:total_spent"`
	VisitCount int64   `gorm:"column:visit_count"`
}

// AtRiskClientRow is a client whose last COMPLETED visit predates a cutoff.
// LastVisit arrives as driver-formatted text (CAST in the query for
// cross-driver portability); the service parses it.
type AtRiskClientRow struct {
	ClientID   uint   `gorm:"column:client_id"`
	ClientName string `gorm:"column:client_name"`
	LastVisit  string `gorm:"column:last_visit"`
}

// BookedSlotRow is one busy interval of ONE professional feeding the occupancy
// computation; durations are parsed from HH:MM strings in Go (design decision
// D4 — driver-portable, no dialect SQL). The service builds these rows from
// the Phase 2a segment kernel, so a split booking yields one row per
// professional instead of the lead's whole envelope.
type BookedSlotRow struct {
	PersonnelID uint
	StartTime   string
	EndTime     string
}

// ReviewPointRow is one raw review feeding the rating trend aggregation.
type ReviewPointRow struct {
	CreatedAt time.Time
	Rating    int
}

// DashboardRepository centralizes the aggregate queries backing the admin
// dashboard stats endpoint. Date boundaries are computed by the service in UTC
// (design decision D2) and passed in, keeping every query deterministic.
type DashboardRepository interface {
	// GetActiveCenter returns the beauty center when it exists and is active;
	// otherwise gorm.ErrRecordNotFound (soft-deleted rows are excluded too).
	GetActiveCenter(centerID uint) (*models.BeautyCenter, error)
	// CountAppointmentsToday counts reservations (status != CANCELLED) with
	// date in [from, to) for the center.
	CountAppointmentsToday(centerID uint, from, to time.Time) (int64, error)
	// CountPending counts reservations with status PENDING for the center.
	CountPending(centerID uint) (int64, error)
	// CountActiveServices counts services with is_active == true.
	CountActiveServices(centerID uint) (int64, error)
	// CountActivePersonnel counts personnel with is_active == true.
	CountActivePersonnel(centerID uint) (int64, error)
	// CountReviews counts reviews for the center.
	CountReviews(centerID uint) (int64, error)
	// AverageRating returns the mean review rating, 0.0 when there are no
	// reviews.
	AverageRating(centerID uint) (float64, error)
	// RevenueBetween sums total_price of COMPLETED reservations with date in
	// [from, to).
	RevenueBetween(centerID uint, from, to time.Time) (float64, error)
	// CompletedAggregates returns total revenue and reservation count for all
	// COMPLETED reservations of the center (average-ticket inputs).
	CompletedAggregates(centerID uint) (revenue float64, count int64, err error)
	// TopServices returns the top `limit` services by reservation count,
	// excluding reservations with status CANCELLED, most booked first.
	TopServices(centerID uint, limit int) ([]TopServiceRow, error)
	// UpcomingReservations returns the next `limit` reservations with status
	// PENDING or CONFIRMED and date >= from, sorted by date then start_time,
	// with the Client preloaded for display.
	UpcomingReservations(centerID uint, from time.Time, limit int) ([]models.Reservation, error)
	// RevenueByDay returns daily revenue (SUM total_price) for COMPLETED
	// reservations with date in [from, to), grouped by date.
	RevenueByDay(centerID uint, from, to time.Time) ([]RevenueDayRow, error)
	// AppointmentsByDay returns daily appointment counts (status != CANCELLED)
	// with date in [from, to), grouped by date.
	AppointmentsByDay(centerID uint, from, to time.Time) ([]CountDayRow, error)
	// HeatmapRows returns the raw COMPLETED reservations (date + start_time)
	// with date in [from, to); the service builds the 7×24 matrix in Go (D4).
	HeatmapRows(centerID uint, from, to time.Time) ([]HeatmapRow, error)
	// CountStatusBetween returns total and CANCELLED reservation counts with
	// date in [from, to) — cancellation-rate inputs in one query.
	CountStatusBetween(centerID uint, from, to time.Time) (CountStatusRow, error)
	// TopServicesByRevenue is the range-scoped leaderboard: SUM of
	// reservation_services.price joined to COMPLETED reservations in
	// [from, to), ranked revenue DESC (ties: appointment_count DESC, then id).
	TopServicesByRevenue(centerID uint, from, to time.Time, limit int) ([]TopServiceRow, error)
	// PersonnelRevenue ranks staff by SUM(total_price) of COMPLETED
	// reservations in [from, to), revenue DESC, limited to `limit` entries.
	PersonnelRevenue(centerID uint, from, to time.Time, limit int) ([]PersonnelRevenueRow, error)

	// ── feat-admin-analytics Phase 3 ────────────────────────────────

	// ClientRecurrence counts NEW clients (earliest non-CANCELLED reservation
	// inside [from, to)) vs RECURRING (visited in range, first visit earlier).
	ClientRecurrence(centerID uint, from, to time.Time) (ClientRecurrenceRow, error)
	// TopClientsBySpend ranks clients by SUM(total_price) of COMPLETED
	// reservations in [from, to), spend DESC, limited to `limit` entries.
	TopClientsBySpend(centerID uint, from, to time.Time, limit int) ([]ClientSpendRow, error)
	// AtRiskClients returns clients whose last COMPLETED visit is strictly
	// before cutoff, oldest first, limited to `limit` entries.
	AtRiskClients(centerID uint, cutoff time.Time, limit int) ([]AtRiskClientRow, error)
	// BookedSlots returns the raw non-CANCELLED reservations in [from, to)
	// with their service lines preloaded (one bounded preload, never a query
	// per reservation). The service derives each professional's busy intervals
	// from the Phase 2a segment kernel — the lead's envelope alone is no longer
	// enough once a booking can be split across professionals.
	BookedSlots(centerID uint, from, to time.Time) ([]models.Reservation, error)
	// PersonnelNames resolves display names for the given personnel ids,
	// scoped to the center; missing/soft-deleted ids are absent from the map.
	PersonnelNames(centerID uint, ids []uint) (map[uint]string, error)
	// WorkingHoursRows returns the center-level schedule rows used as the
	// shared availability denominator for occupancy.
	WorkingHoursRows(centerID uint) ([]models.WorkingHour, error)
	// CompletedCountByDay returns daily COMPLETED counts grouped by date —
	// average-ticket-trend denominator.
	CompletedCountByDay(centerID uint, from, to time.Time) ([]CountDayRow, error)
	// ReviewPoints returns raw review ratings with timestamps in [from, to);
	// day-bucketing happens in Go (D4 portability).
	ReviewPoints(centerID uint, from, to time.Time) ([]ReviewPointRow, error)
}
