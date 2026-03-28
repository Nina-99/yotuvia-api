package repository

import "github.com/Nina-99/yottu-api/models"

type FeedPostRepository interface {
	Create(post *models.FeedPost) error
	Update(post *models.FeedPost) error
	Delete(id uint) error
	FindByID(id uint) (*models.FeedPost, error)
	FindAll() ([]models.FeedPost, error)
	FindByBeautyCenterID(centerID uint) ([]models.FeedPost, error)
}
