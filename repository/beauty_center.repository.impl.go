package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type BeautyCenterRepositoryImpl struct {
	db *gorm.DB
}

func NewBeautyCenterRepositoryImpl(db *gorm.DB) BeautyCenterRepository {
	return &BeautyCenterRepositoryImpl{db: db}
}

// Create implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) Create(center *models.BeautyCenter) error {
	return r.db.Create(center).Error
}

// Update implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) Update(center *models.BeautyCenter) error {
	return r.db.Save(center).Error
}

// Delete implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.BeautyCenter{}, id).Error
}

// FindByID implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) FindByID(id uint) (*models.BeautyCenter, error) {
	var center models.BeautyCenter
	err := r.db.Preload("WorkingHour").Preload("BeautyCenterImage").First(&center, id).Error
	if err != nil {
		return nil, err
	}
	return &center, nil
}

// FindAll implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) FindAll() ([]models.BeautyCenter, error) {
	var centers []models.BeautyCenter
	err := r.db.Preload("WorkingHour").Preload("BeautyCenterImage").Find(&centers).Error
	return centers, err
}

// FindByEmail implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) FindByEmail(email string) (*models.BeautyCenter, error) {
	var center models.BeautyCenter
	err := r.db.Where("email = ?", email).First(&center).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &center, nil
}
