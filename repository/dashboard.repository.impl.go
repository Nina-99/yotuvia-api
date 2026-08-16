package repository

import (
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