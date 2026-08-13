package repository

import (
	"errors"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type ReservationRepositoryImpl struct {
	db *gorm.DB
}

func NewReservationRepositoryImpl(db *gorm.DB) ReservationRepository {
	return &ReservationRepositoryImpl{db: db}
}

func (r *ReservationRepositoryImpl) Create(reservation *models.Reservation) error {
	return r.db.Create(reservation).Error
}

func (r *ReservationRepositoryImpl) Update(reservation *models.Reservation) error {
	return r.db.Save(reservation).Error
}

func (r *ReservationRepositoryImpl) Delete(reservation *models.Reservation) error {
	return r.db.Delete(reservation).Error
}

func (r *ReservationRepositoryImpl) FindById(reservationID uint) (*models.Reservation, error) {
	var reservation models.Reservation
	err := r.db.
		Preload("ReservationServices.Service.Category").Preload("Client").Preload("Personnel").Preload("BeautyCenter").
		First(&reservation, reservationID).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("reservation not found")
		}
		return nil, err
	}
	return &reservation, nil
}

func (r *ReservationRepositoryImpl) FindUpcomingByClientID(clientID uint) (*models.Reservation, error) {
	var reservation models.Reservation
	today := time.Now().Format("2006-01-02")
	err := r.db.
		Preload("ReservationServices").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("client_id = ? AND date >= ? AND status NOT IN (?)", clientID, today, []string{"COMPLETED", "CANCELLED"}).
		Order("date ASC, start_time ASC").
		First(&reservation).Error
	if err != nil {
		return nil, err
	}
	return &reservation, nil
}

func (r *ReservationRepositoryImpl) FindByClientID(clientID uint) ([]models.Reservation, error) {
	var reservations []models.Reservation
	err := r.db.
		Preload("ReservationServices.Service.Category").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("client_id = ?", clientID).
		Order("date DESC, start_time DESC").
		Find(&reservations).Error
	return reservations, err
}

func (r *ReservationRepositoryImpl) FindByBeautyCenterId(beautyCenterID uint) ([]models.Reservation, error) {
	var reservations []models.Reservation
	err := r.db.
		Preload("ReservationServices").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("beauty_center_id = ?", beautyCenterID).
		Find(&reservations).Error
	return reservations, err
}

// FindByPersonnelId implements ReservationRepository. The where clause is
// assembled conditionally so nil from/to stay open-ended while GORM still
// handles the bound parameters.
func (r *ReservationRepositoryImpl) FindByPersonnelId(personnelID uint, from, to *time.Time) ([]models.Reservation, error) {
	var reservations []models.Reservation
	query := r.db.
		Preload("ReservationServices").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("personnel_id = ?", personnelID)
	if from != nil {
		query = query.Where("date >= ?", *from)
	}
	if to != nil {
		query = query.Where("date <= ?", *to)
	}
	err := query.Order("date ASC, start_time ASC").Find(&reservations).Error
	return reservations, err
}

func (r *ReservationRepositoryImpl) FindAll(page, limit int, centerID uint) ([]models.Reservation, int64, error) {
	var reservations []models.Reservation
	var total int64

	base := r.db.Model(&models.Reservation{}).Where("beauty_center_id = ?", centerID)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.
		Preload("ReservationServices").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("beauty_center_id = ?", centerID).
		Offset(offset).
		Limit(limit).
		Find(&reservations).Error
	return reservations, total, err
}

func (r *ReservationRepositoryImpl) CheckOverlap(centerID uint, date time.Time, startTime, endTime string, personnelID, excludeID uint) (bool, error) {
	var count int64
	query := r.db.Model(&models.Reservation{}).
		Where("beauty_center_id = ? AND date = ? AND status != 'CANCELLED'", centerID, date).
		Where("((start_time < ? AND end_time > ?) OR (start_time >= ? AND start_time < ?) OR (end_time > ? AND end_time <= ?))",
			endTime, startTime, startTime, endTime, startTime, endTime)

	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}

	if personnelID > 0 {
		query = query.Where("personnel_id = ?", personnelID)
	}

	err := query.Count(&count).Error
	return count > 0, err
}

func (r *ReservationRepositoryImpl) UpdatePersonnelID(reservationID uint, personnelID uint) error {
	return r.db.Model(&models.Reservation{}).
		Where("id = ?", reservationID).
		Update("personnel_id", personnelID).Error
}

func (r *ReservationRepositoryImpl) DeleteReservationServices(reservationID uint) error {
	return r.db.Where("reservation_id = ?", reservationID).Delete(&models.ReservationService{}).Error
}

func (r *ReservationRepositoryImpl) FindByToken(token string) (*models.Reservation, error) {
	var reservation models.Reservation
	err := r.db.
		Preload("ReservationServices").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("verification_token = ?", token).
		First(&reservation).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &reservation, nil
}

// CompleteVerification performs the atomic exactly-once update: only a
// reservation that is still PENDING or CONFIRMED is completed, the token and
// expiry are cleared, and the status is persisted in canonical UPPERCASE.
func (r *ReservationRepositoryImpl) CompleteVerification(reservation *models.Reservation) (bool, error) {
	result := r.db.Model(&models.Reservation{}).
		Where("id = ? AND status IN ?", reservation.ID, []string{"PENDING", "CONFIRMED"}).
		Updates(map[string]interface{}{
			"status":             "COMPLETED",
			"verification_token": gorm.Expr("NULL"),
			"token_expires_at":   gorm.Expr("NULL"),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
