package repository

import (
	"time"

	"github.com/Nina-99/yottu-api/models"
)

// TopServiceRow is the aggregated leaderboard row returned by TopServices: a
// service's id/name snapshot plus how many reservations reference it.
type TopServiceRow struct {
	ServiceID        uint   `gorm:"column:service_id"`
	ServiceName      string `gorm:"column:service_name"`
	AppointmentCount int64  `gorm:"column:appointment_count"`
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
}