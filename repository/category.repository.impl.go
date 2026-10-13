package repository

import (
	"errors"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type CategoryRepositoryImpl struct {
	db *gorm.DB
}

func NewCategoryRepositoryImpl(db *gorm.DB) CategoryRepository {
	return &CategoryRepositoryImpl{db: db}
}

func (r *CategoryRepositoryImpl) Create(category *models.Category) error {
	return r.db.Create(category).Error
}

func (r *CategoryRepositoryImpl) Update(category *models.Category) error {
	return r.db.Save(category).Error
}

func (r *CategoryRepositoryImpl) Delete(category *models.Category) error {
	return r.db.Delete(category).Error
}

func (r *CategoryRepositoryImpl) FindById(categoryID uint) (*models.Category, error) {
	var category models.Category
	err := r.db.Preload("Services").First(&category, categoryID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("category not found")
		}
		return nil, err
	}
	return &category, nil
}

func (r *CategoryRepositoryImpl) FindAll(beautyCenterID uint, page, limit int) ([]models.Category, int64, error) {
	var categories []models.Category
	var total int64

	countQuery := r.db.Model(&models.Category{})
	fetchQuery := r.db.Preload("Services")

	if beautyCenterID != 0 {
		countQuery = countQuery.Where("beauty_center_id = ?", beautyCenterID)
		fetchQuery = fetchQuery.Where("beauty_center_id = ?", beautyCenterID)
	}

	countQuery.Count(&total)
	offset := (page - 1) * limit
	err := fetchQuery.Offset(offset).Limit(limit).Find(&categories).Error
	if err != nil {
		return nil, total, err
	}
	return categories, total, nil
}
