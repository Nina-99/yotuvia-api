package repository

import (
	"time"

	"github.com/Nina-99/yottu-api/models"
)

type BeautyCenterRepository interface {
	Create(center *models.BeautyCenter) error
	Update(center *models.BeautyCenter) error
	Delete(id uint) error
	FindByID(id uint) (*models.BeautyCenter, error)
	FindAll(page, limit int) ([]models.BeautyCenter, int64, error)
	FindAllActiveForClients() ([]models.BeautyCenter, error)
	FindByEmail(email string) (*models.BeautyCenter, error)
	ReplaceSchedules(centerID uint, schedules []models.WorkingHour) error
	ReplaceImages(centerID uint, images []models.BeautyCenterImage) error
	FindWorkingHours(centerID uint, dayOfWeek int) ([]models.WorkingHour, error)
	FindReservationsForDate(centerID uint, date time.Time, personnelID uint) ([]models.Reservation, error)
	Search(query string, page, limit int) ([]models.BeautyCenter, int64, error)
	FindTopRated(limit int) ([]models.BeautyCenter, error)
	FindNearby(latitude, longitude float64, limit int) ([]models.BeautyCenter, error)
	FindByReferralCode(code string) (*models.BeautyCenter, error)
}
