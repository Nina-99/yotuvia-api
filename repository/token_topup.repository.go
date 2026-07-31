package repository

import "github.com/Nina-99/yottu-api/models"

type TokenTopUpRepository interface {
	Create(topup *models.TokenTopUp) error
	Update(topup *models.TokenTopUp) error
	FindByID(id uint) (*models.TokenTopUp, error)
	FindByBeautyCenterID(beautyCenterID uint) ([]models.TokenTopUp, error)
	FindPending() ([]models.TokenTopUp, error)
	FindAll(page, limit int) ([]models.TokenTopUp, int64, error)
}
