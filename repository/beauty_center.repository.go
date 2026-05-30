package repository

import "github.com/Nina-99/yottu-api/models"

type BeautyCenterRepository interface {
	Create(center *models.BeautyCenter) error
	Update(center *models.BeautyCenter) error
	Delete(id uint) error
	FindByID(id uint) (*models.BeautyCenter, error)
	FindAll() ([]models.BeautyCenter, error)
	FindAllActiveForClients() ([]models.BeautyCenter, error)
	FindByEmail(email string) (*models.BeautyCenter, error)
}
