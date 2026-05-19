package repository

import "github.com/Nina-99/yottu-api/models"

type BeautyCenterImageRepository interface {
	Save(BeautyCenterImage *models.BeautyCenterImage) error
	FindByID(id uint) (*models.BeautyCenterImage, error)
	FindByBeautyCenterID(beautyID uint) ([]models.BeautyCenterImage, error)
	Delete(id uint) error
}
