package service

import (
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
)

type CategoryService interface {
	Create(req request.CreateCategoryRequest) error
	Update(categoryID uint, req request.CreateCategoryRequest) error
	Delete(categoryID uint) error
	FindAll(beautyCenterID uint, page, limit int) ([]models.Category, int64, error)
	FindById(categoryID uint) (*models.Category, error)
}
