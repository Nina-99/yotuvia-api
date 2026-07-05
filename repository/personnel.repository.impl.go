package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type PersonnelRepositoryImpl struct {
	db *gorm.DB
}

func NewPersonnelRepositoryImpl(db *gorm.DB) PersonnelRepository {
	return &PersonnelRepositoryImpl{db: db}
}

// Create implements PersonnelRepository
func (r *PersonnelRepositoryImpl) Create(personnel *models.Personnel) error {
	return r.db.Create(personnel).Error
}

// Update implements PersonnelRepository
func (r *PersonnelRepositoryImpl) Update(personnel *models.Personnel) error {
	return r.db.Save(personnel).Error
}

// Delete implements PersonnelRepository
func (r *PersonnelRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.Personnel{}, id).Error
}

// FindByID implements PersonnelRepository
func (r *PersonnelRepositoryImpl) FindByID(id uint) (*models.Personnel, error) {
	var personnel models.Personnel
	err := r.db.Preload("PersonnelImage").Preload("BeautyCenter").First(&personnel, id).Error
	if err != nil {
		return nil, err
	}
	return &personnel, nil
}

// FindAll implements PersonnelRepository
func (r *PersonnelRepositoryImpl) FindAll(beautyCenterID uint, page, limit int) ([]models.Personnel, int64, error) {
	var personnel []models.Personnel
	var total int64

	countDb := r.db.Model(&models.Personnel{})
	fetchDb := r.db.Preload("PersonnelImage").Preload("BeautyCenter")
	if beautyCenterID != 0 {
		countDb = countDb.Where("beauty_center_id = ?", beautyCenterID)
		fetchDb = fetchDb.Where("beauty_center_id = ?", beautyCenterID)
	}
	countDb.Count(&total)

	offset := (page - 1) * limit
	err := fetchDb.Offset(offset).Limit(limit).Find(&personnel).Error
	return personnel, total, err
}

// FindByEmail implements PersonnelRepository
func (r *PersonnelRepositoryImpl) FindByEmail(email string) (*models.Personnel, error) {
	var personnel models.Personnel
	err := r.db.Where("email = ?", email).First(&personnel).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &personnel, nil
}
