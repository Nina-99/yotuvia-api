package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
)

type CategoryService interface {
	Create(req request.CreateCategoryRequest) error
	Update(categoryID uint, req request.CreateCategoryRequest) error
	Delete(categoryID uint) error
	FindAll() ([]models.Category, error)
	FindById(categoryID uint) (*models.Category, error)
}
