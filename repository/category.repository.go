package repository

import "github.com/Nina-99/yotuvia-api/models"

type CategoryRepository interface {
	Create(category *models.Category) error
	Update(category *models.Category) error
	Delete(category *models.Category) error
	FindById(categoryID uint) (*models.Category, error)
	FindAll(beautyCenterID uint, page, limit int) ([]models.Category, int64, error)
}
