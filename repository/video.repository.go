package repository

import "github.com/Nina-99/yottu-api/models"

type VideoRepository interface {
	Create(video *models.Video) error
	Delete(video *models.Video) error
	FindByID(id uint) (*models.Video, error)
	FindAll(page, limit int) ([]models.Video, int64, error)
	FindByBeautyCenterID(centerID uint, page, limit int) ([]models.Video, int64, error)
	IncrementViews(id uint) error
}
