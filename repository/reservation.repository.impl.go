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

// Create implements ClientRepository
func (r *ReservationRepositoryImpl) Create(reservation *models.Reservation) error {
	return r.db.Create(&reservation).Error
}

// Update implements ClientRepository
func (r *ReservationRepositoryImpl) Update(reservation *models.Reservation) error {
	return r.db.Save(reservation).Error
}

// Delete implements ClientRepository
func (r *ReservationRepositoryImpl) Delete(reservation *models.Reservation) error {
	return r.db.Delete(&reservation).Error
}

// FindById implements ClientRepository
func (r *ReservationRepositoryImpl) FindById(reservationID uint) (*models.Reservation, error) {
	var reservation models.Reservation
	err := r.db.Preload("Client").Preload("Personnel").Preload("Service").Preload("BeautyCenter").First(&reservation, reservationID).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("reservation not found")
		}
		return nil, err
	}
	return &reservation, nil
}

// FindClientID implements ReservationRepository
func (r *ReservationRepositoryImpl) FindClientID(clientID uint) (*models.Reservation, error) {
	var reservation models.Reservation
	err := r.db.Where("client_id = ?", clientID).First(&reservation).Error
	return &reservation, err
}

// FindBeautyCenterId implements ReservationRepository
func (r *ReservationRepositoryImpl) FindBeautyCenterId(beautyCenterID uint) (*models.Reservation, error) {
	var reservation models.Reservation
	err := r.db.Where("beauty_center_id = ?", beautyCenterID).First(&reservation).Error
	return &reservation, err
}

// FindAll implements ClientRepository
func (r *ReservationRepositoryImpl) FindAll() ([]models.Reservation, error) {
	var reservations []models.Reservation
	err := r.db.Preload("Client").Preload("Personnel").Preload("Service").Preload("BeautyCenter").Find(&reservations).Error
	return reservations, err
}

func (r *ReservationRepositoryImpl) CheckOverlap(centerID uint, date time.Time, startTime, endTime string, excludeID uint) (bool, error) {
	var count int64
	query := r.db.Model(&models.Reservation{}).
		Where("beauty_center_id = ? AND date = ? AND status != 'CANCELLED'", centerID, date).
		Where("((start_time < ? AND end_time > ?) OR (start_time >= ? AND start_time < ?) OR (end_time > ? AND end_time <= ?))",
			endTime, startTime, startTime, endTime, startTime, endTime)

	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}

	err := query.Count(&count).Error
	return count > 0, err
}
