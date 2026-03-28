package repository

import "github.com/Nina-99/yottu-api/models"

type CategoryRepository interface {
	Create(category *models.Category) error
	Update(category *models.Category) error
	Delete(category *models.Category) error
	FindById(categoryID uint) (*models.Category, error)
	FindAll() ([]models.Category, error)
}
