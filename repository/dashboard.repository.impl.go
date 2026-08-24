package repository

import (
	"strings"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

// DashboardRepositoryImpl implements DashboardRepository over GORM. Soft
// delete is enforced two ways: GORM appends `deleted_at IS NULL` automatically
// for the queried model, and joined models (reservations inside
// TopServices/UpcomingReservations) declare it explicitly in the join/where
// clauses. Both PostgreSQL (DATE column) and the SQLite test driver store
// dates as text/timestamps, so time.Time bounds compare consistently.
type DashboardRepositoryImpl struct {
	db *gorm.DB
}

func NewDashboardRepositoryImpl(db *gorm.DB) DashboardRepository {
	return &DashboardRepositoryImpl{db: db}
}

func (r *DashboardRepositoryImpl) GetActiveCenter(centerID uint) (*models.BeautyCenter, error) {
	var center models.BeautyCenter
	err := r.db.Where("id = ? AND is_active = ?", centerID, true).First(&center).Error
	if err != nil {
		return nil, err
	}
	return &center, nil
}

func (r *DashboardRepositoryImpl) CountAppointmentsToday(centerID uint, from, to time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.Reservation{}).
		Where("beauty_center_id = ? AND status <> ? AND date >= ? AND date < ?", centerID, "CANCELLED", from, to).
		Count(&count).Error
	return count, err
}

func (r *DashboardRepositoryImpl) CountPending(centerID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Reservation{}).
		Where("beauty_center_id = ? AND status = ?", centerID, "PENDING").
		Count(&count).Error
	return count, err
}

func (r *DashboardRepositoryImpl) CountActiveServices(centerID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Service{}).
		Where("beauty_center_id = ? AND is_active = ?", centerID, true).
		Count(&count).Error
	return count, err
}

func (r *DashboardRepositoryImpl) CountActivePersonnel(centerID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Personnel{}).
		Where("beauty_center_id = ? AND is_active = ?", centerID, true).
		Count(&count).Error
	return count, err
}

func (r *DashboardRepositoryImpl) CountReviews(centerID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Review{}).
		Where("beauty_center_id = ?", centerID).
		Count(&count).Error
	return count, err
}

func (r *DashboardRepositoryImpl) AverageRating(centerID uint) (float64, error) {
	var avg float64
	err := r.db.Model(&models.Review{}).
		Where("beauty_center_id = ?", centerID).
		Select("COALESCE(AVG(rating), 0)").
		Scan(&avg).Error
	return avg, err
}

func (r *DashboardRepositoryImpl) RevenueBetween(centerID uint, from, to time.Time) (float64, error) {
	var revenue float64
	err := r.db.Model(&models.Reservation{}).
		Where("beauty_center_id = ? AND status = ? AND date >= ? AND date < ?", centerID, "COMPLETED", from, to).
		Select("COALESCE(SUM(total_price), 0)").
		Scan(&revenue).Error
	return revenue, err
}

func (r *DashboardRepositoryImpl) CompletedAggregates(centerID uint) (float64, int64, error) {
	var agg struct {
		Revenue float64
		Count   int64
	}
	err := r.db.Model(&models.Reservation{}).
		Where("beauty_center_id = ? AND status = ?", centerID, "COMPLETED").
		Select("COALESCE(SUM(total_price), 0) AS revenue, COUNT(*) AS count").
		Scan(&agg).Error
	return agg.Revenue, agg.Count, err
}

// TopServices counts reservation_service rows joined against their (live)
// reservations. GORM auto-filters soft-deleted reservation_services; the join
// condition explicitly excludes soft-deleted reservations because GORM never
// adds deleted_at clauses to raw join conditions.
func (r *DashboardRepositoryImpl) TopServices(centerID uint, limit int) ([]TopServiceRow, error) {
	var rows []TopServiceRow
	err := r.db.Model(&models.ReservationService{}).
		Select("reservation_services.service_id, reservation_services.service_name, COUNT(*) AS appointment_count").
		Joins("JOIN reservations ON reservations.id = reservation_services.reservation_id AND reservations.deleted_at IS NULL").
		Where("reservations.beauty_center_id = ? AND reservations.status <> ?", centerID, "CANCELLED").
		Group("reservation_services.service_id, reservation_services.service_name").
		Order("appointment_count DESC, reservation_services.service_id ASC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

// UpcomingReservations preloads each reservation's Client (both models are
// soft-delete filtered by GORM automatically) and returns the nearest slots.
func (r *DashboardRepositoryImpl) UpcomingReservations(centerID uint, from time.Time, limit int) ([]models.Reservation, error) {
	var reservations []models.Reservation
	err := r.db.
		Preload("Client").
		Where("beauty_center_id = ? AND status IN ? AND date >= ?", centerID, []string{"PENDING", "CONFIRMED"}, from).
		Order("date ASC, start_time ASC, id ASC").
		Limit(limit).
		Find(&reservations).Error
	return reservations, err
}

// RevenueByDay returns daily revenue (SUM total_price) for COMPLETED
// reservations with date in [from, to), grouped by date.
func (r *DashboardRepositoryImpl) RevenueByDay(centerID uint, from, to time.Time) ([]RevenueDayRow, error) {
	var rows []RevenueDayRow
	err := r.db.Model(&models.Reservation{}).
		Where("beauty_center_id = ? AND status = ? AND date >= ? AND date < ?", centerID, "COMPLETED", from, to).
		Select("date, COALESCE(SUM(total_price), 0) AS value").
		Group("date").
		Order("date ASC").
		Scan(&rows).Error
	return rows, err
}

// AppointmentsByDay returns daily appointment counts (status != CANCELLED)
// with date in [from, to), grouped by date.
func (r *DashboardRepositoryImpl) AppointmentsByDay(centerID uint, from, to time.Time) ([]CountDayRow, error) {
	var rows []CountDayRow
	err := r.db.Model(&models.Reservation{}).
		Where("beauty_center_id = ? AND status <> ? AND date >= ? AND date < ?", centerID, "CANCELLED", from, to).
		Select("date, COUNT(*) AS value").
		Group("date").
		Order("date ASC").
		Scan(&rows).Error
	return rows, err
}

// HeatmapRows returns the minimal raw rows feeding the day×hour heatmap: one
// per COMPLETED reservation in [from, to). No GROUP BY — the service layer
// aggregates in Go so the query stays free of dialect-specific functions
// (design decision D4: PostgreSQL vs SQLite test driver portability).
func (r *DashboardRepositoryImpl) HeatmapRows(centerID uint, from, to time.Time) ([]HeatmapRow, error) {
	var rows []HeatmapRow
	err := r.db.Model(&models.Reservation{}).
		Where("beauty_center_id = ? AND status = ? AND date >= ? AND date < ?", centerID, "COMPLETED", from, to).
		Select("date, start_time").
		Order("date ASC, start_time ASC").
		Scan(&rows).Error
	return rows, err
}

// CountStatusBetween returns total and CANCELLED reservation counts for
// [from, to) in a single pass. SUM(CASE …) is plain SQL, portable across both
// supported drivers.
func (r *DashboardRepositoryImpl) CountStatusBetween(centerID uint, from, to time.Time) (CountStatusRow, error) {
	var row CountStatusRow
	err := r.db.Model(&models.Reservation{}).
		Where("beauty_center_id = ? AND date >= ? AND date < ?", centerID, from, to).
		Select("COUNT(*) AS total, COALESCE(SUM(CASE WHEN status = 'CANCELLED' THEN 1 ELSE 0 END), 0) AS cancelled").
		Scan(&row).Error
	return row, err
}

// TopServicesByRevenue is the range-scoped leaderboard backing the analytics
// surface: revenue is SUM(reservation_services.price) — the price captured at
// booking time — over COMPLETED reservations only. Ranking flips the legacy
// count-based order deliberately (design decision D5): a cheap-but-frequent
// service ranks below an expensive-but-rare one. Ties fall back to
// appointment count then stable id order.
func (r *DashboardRepositoryImpl) TopServicesByRevenue(centerID uint, from, to time.Time, limit int) ([]TopServiceRow, error) {
	var rows []TopServiceRow
	err := r.db.Model(&models.ReservationService{}).
		Select(`reservation_services.service_id,
			       reservation_services.service_name,
			       SUM(reservation_services.price) AS revenue,
			       COUNT(DISTINCT reservation_services.reservation_id) AS appointment_count`).
		Joins("JOIN reservations ON reservations.id = reservation_services.reservation_id AND reservations.deleted_at IS NULL").
		Where("reservations.beauty_center_id = ? AND reservations.status = ? AND reservations.date >= ? AND reservations.date < ?", centerID, "COMPLETED", from, to).
		Group("reservation_services.service_id, reservation_services.service_name").
		Order("revenue DESC, appointment_count DESC, reservation_services.service_id ASC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

// PersonnelRevenue ranks staff by COMPLETED revenue in [from, to). The join
// excludes soft-deleted personnel explicitly (GORM never adds deleted_at to
// raw join conditions — same precedent as TopServices).
func (r *DashboardRepositoryImpl) PersonnelRevenue(centerID uint, from, to time.Time, limit int) ([]PersonnelRevenueRow, error) {
	var rows []PersonnelRevenueRow
	err := r.db.Model(&models.Reservation{}).
		Select(`reservations.personnel_id,
			       COALESCE(personnels.first_name || ' ' || personnels.last_name, '') AS personnel_name,
			       COALESCE(SUM(reservations.total_price), 0) AS revenue,
			       COUNT(*) AS appointment_count`).
		Joins("JOIN personnels ON personnels.id = reservations.personnel_id AND personnels.deleted_at IS NULL").
		Where("reservations.beauty_center_id = ? AND reservations.status = ? AND reservations.date >= ? AND reservations.date < ?", centerID, "COMPLETED", from, to).
		Group("reservations.personnel_id, personnels.first_name, personnels.last_name").
		Order("revenue DESC, reservations.personnel_id ASC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

// ClientRecurrence classifies every client with at least one non-CANCELLED
// reservation: NEW when their all-time first visit falls inside [from, to),
// RECURRING when they visited in range but their first visit predates it.
// Clients whose first visit predates the range without visiting in it are
// lapsed — counted by neither bucket. Cancelled bookings never establish a
// "visit". Plain subquery SQL, portable across both drivers.
func (r *DashboardRepositoryImpl) ClientRecurrence(centerID uint, from, to time.Time) (ClientRecurrenceRow, error) {
	var row ClientRecurrenceRow
	err := r.db.Raw(`
		SELECT
			COALESCE(SUM(CASE WHEN t.first_date >= ? AND t.first_date < ? THEN 1 ELSE 0 END), 0) AS new_clients,
			COALESCE(SUM(CASE WHEN t.first_date < ? THEN 1 ELSE 0 END), 0) AS recurring_clients
		FROM (
			SELECT client_id, MIN(date) AS first_date
			FROM reservations
			WHERE beauty_center_id = ? AND status <> 'CANCELLED' AND deleted_at IS NULL
			GROUP BY client_id
		) t
		WHERE EXISTS (
			SELECT 1 FROM reservations r2
			WHERE r2.client_id = t.client_id
			  AND r2.beauty_center_id = ?
			  AND r2.status <> 'CANCELLED'
			  AND r2.deleted_at IS NULL
			  AND r2.date >= ? AND r2.date < ?
		)`,
		from, to, from, centerID, centerID, from, to,
	).Scan(&row).Error
	return row, err
}

// TopClientsBySpend ranks clients by COMPLETED revenue in [from, to). The
// clients join excludes soft-deleted clients explicitly (raw join condition).
func (r *DashboardRepositoryImpl) TopClientsBySpend(centerID uint, from, to time.Time, limit int) ([]ClientSpendRow, error) {
	var rows []ClientSpendRow
	err := r.db.Table("reservations").
		Select(`reservations.client_id,
			       COALESCE(clients.first_name || ' ' || clients.last_name, '') AS client_name,
			       COALESCE(SUM(reservations.total_price), 0) AS total_spent,
			       COUNT(*) AS visit_count`).
		Joins("JOIN clients ON clients.id = reservations.client_id AND clients.deleted_at IS NULL").
		Where("reservations.beauty_center_id = ? AND reservations.status = ? AND reservations.date >= ? AND reservations.date < ?", centerID, "COMPLETED", from, to).
		Group("reservations.client_id, clients.first_name, clients.last_name").
		Order("total_spent DESC, reservations.client_id ASC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

// AtRiskClients returns clients whose last COMPLETED visit is strictly before
// the cutoff (last_visit < cutoff → a visit exactly cutoff days ago is NOT at
// risk). Oldest visits first so the most lapsed clients surface on top.
// MAX(date) is CAST to TEXT so both PostgreSQL and the SQLite driver return a
// scannable string (aggregate columns bypass GORM's column typing).
func (r *DashboardRepositoryImpl) AtRiskClients(centerID uint, cutoff time.Time, limit int) ([]AtRiskClientRow, error) {
	var rows []AtRiskClientRow
	err := r.db.Table("reservations").
		Select(`reservations.client_id,
			       COALESCE(clients.first_name || ' ' || clients.last_name, '') AS client_name,
			       CAST(MAX(reservations.date) AS TEXT) AS last_visit`).
		Joins("JOIN clients ON clients.id = reservations.client_id AND clients.deleted_at IS NULL").
		Where("reservations.beauty_center_id = ? AND reservations.status = ?", centerID, "COMPLETED").
		Group("reservations.client_id, clients.first_name, clients.last_name").
		Having("MAX(reservations.date) < ?", cutoff).
		Order("last_visit ASC, reservations.client_id ASC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

// BookedSlots returns raw non-CANCELLED slots in [from, to); the service
// parses HH:MM durations in Go (D4 portability).
func (r *DashboardRepositoryImpl) BookedSlots(centerID uint, from, to time.Time) ([]BookedSlotRow, error) {
	var rows []BookedSlotRow
	err := r.db.Model(&models.Reservation{}).
		Where("beauty_center_id = ? AND status <> ? AND date >= ? AND date < ?", centerID, "CANCELLED", from, to).
		Select("personnel_id, start_time, end_time").
		Scan(&rows).Error
	return rows, err
}

// PersonnelNames resolves display names for occupancy rows. GORM filters
// soft-deleted personnel automatically on the base model.
func (r *DashboardRepositoryImpl) PersonnelNames(centerID uint, ids []uint) (map[uint]string, error) {
	names := make(map[uint]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	var rows []models.Personnel
	err := r.db.Where("beauty_center_id = ? AND id IN ?", centerID, ids).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, p := range rows {
		names[p.ID] = strings.TrimSpace(p.FirstName + " " + p.LastName)
	}
	return names, nil
}

// WorkingHoursRows returns the center-level schedule rows. NOTE: schedules
// belong to the CENTER (no per-personnel granularity exists in the schema),
// so occupancy uses them as a shared availability denominator across staff.
func (r *DashboardRepositoryImpl) WorkingHoursRows(centerID uint) ([]models.WorkingHour, error) {
	var rows []models.WorkingHour
	err := r.db.Where("beauty_center_id = ?", centerID).Find(&rows).Error
	return rows, err
}

// CompletedCountByDay returns daily COMPLETED counts grouped by date —
// denominator of the average-ticket trend series.
func (r *DashboardRepositoryImpl) CompletedCountByDay(centerID uint, from, to time.Time) ([]CountDayRow, error) {
	var rows []CountDayRow
	err := r.db.Model(&models.Reservation{}).
		Where("beauty_center_id = ? AND status = ? AND date >= ? AND date < ?", centerID, "COMPLETED", from, to).
		Select("date, COUNT(*) AS value").
		Group("date").
		Order("date ASC").
		Scan(&rows).Error
	return rows, err
}

// ReviewPoints returns raw review ratings with timestamps in [from, to);
// day-bucketing happens in Go because created_at date extraction differs
// between PostgreSQL and SQLite (D4 portability precedent).
func (r *DashboardRepositoryImpl) ReviewPoints(centerID uint, from, to time.Time) ([]ReviewPointRow, error) {
	var rows []ReviewPointRow
	err := r.db.Model(&models.Review{}).
		Where("beauty_center_id = ? AND created_at >= ? AND created_at < ?", centerID, from, to).
		Select("created_at, rating").
		Scan(&rows).Error
	return rows, err
}
