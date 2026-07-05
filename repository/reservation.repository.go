package repository

import (
	"time"

	"github.com/Nina-99/yottu-api/models"
)

type ReservationRepository interface {
	Create(reservation *models.Reservation) error
	Update(reservation *models.Reservation) error
	Delete(reservation *models.Reservation) error
	FindById(reservationID uint) (*models.Reservation, error)
	FindClientID(clientID uint) (*models.Reservation, error)
	FindBeautyCenterId(beautyCenterID uint) (*models.Reservation, error)
	FindAll(page, limit int) ([]models.Reservation, int64, error)
	CheckOverlap(centerID uint, date time.Time, startTime, endTime string, excludeID uint) (bool, error)
	FindByToken(token string) (*models.Reservation, error)
}
