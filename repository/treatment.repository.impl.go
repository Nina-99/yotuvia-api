package repository

import (
	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type TreatmentRepositoryImpl struct {
	db *gorm.DB
}

func NewTreatmentRepositoryImpl(db *gorm.DB) TreatmentRepository {
	return &TreatmentRepositoryImpl{db: db}
}

func (r *TreatmentRepositoryImpl) FindCatalog() ([]models.TreatmentCategory, error) {
	var categories []models.TreatmentCategory
	err := r.db.
		Preload("Treatments", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ?", true).Order("name ASC")
		}).
		Order("name ASC").
		Find(&categories).Error
	if err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *TreatmentRepositoryImpl) FindByIDs(ids []uint) ([]models.Treatment, error) {
	var treatments []models.Treatment
	if len(ids) == 0 {
		return []models.Treatment{}, nil
	}
	err := r.db.Where("id IN ? AND is_active = ?", ids, true).Find(&treatments).Error
	if err != nil {
		return nil, err
	}
	return treatments, nil
}
