package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type CategoryRepositoryImpl struct {
	db *gorm.DB
}

func NewCategoryRepositoryImpl(db *gorm.DB) CategoryRepository {
	return &CategoryRepositoryImpl{db: db}
}

// Create implements CategoryRepository
func (r *CategoryRepositoryImpl) Create(category *models.Category) error {
	return r.db.Create(&category).Error
}

// Update implements CategoryRepository
func (r *CategoryRepositoryImpl) Update(category *models.Category) error {
	return r.db.Save(category).Error
}

// Delete implements CategoryRepository
func (r *CategoryRepositoryImpl) Delete(category *models.Category) error {
	return r.db.Delete(&category).Error
}

func (r *CategoryRepositoryImpl) FindById(categoryID uint) (*models.Category, error) {
	var category models.Category
	err := r.db.Preload("Service").First(&category, categoryID).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *CategoryRepositoryImpl) FindAll() ([]models.Category, error) {
	var category []models.Category
	err := r.db.Preload("Service").Find(&category).Error
	if err != nil {
		return nil, err
	}
	return category, nil
}
