package repository

import (
	"errors"
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type PersonnelImageRepositoryImpl struct {
	Db *gorm.DB
}

func NewPersonnelImageRepositoryImpl(Db *gorm.DB) PersonnelImageRepository {
	return &PersonnelImageRepositoryImpl{Db: Db}
}

func (r *PersonnelImageRepositoryImpl) Save(personnelImage *models.PersonnelImage) error {
	result := r.Db.Save(personnelImage)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (r *PersonnelImageRepositoryImpl) FindByID(id uint) (*models.PersonnelImage, error) {
	var personnelImage models.PersonnelImage
	result := r.Db.First(&personnelImage, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("personnel image not found")
		}
		return nil, result.Error
	}
	return &personnelImage, nil
}

func (r *PersonnelImageRepositoryImpl) FindByPersonnelID(personnelID uint) ([]models.PersonnelImage, error) {
	var personnelImages []models.PersonnelImage
	result := r.Db.Where("personnel_id = ?", personnelID).Find(&personnelImages)
	if result.Error != nil {
		return nil, result.Error
	}
	return personnelImages, nil
}

func (r *PersonnelImageRepositoryImpl) Delete(id uint) error {
	var personnelImage models.PersonnelImage
	result := r.Db.Delete(&personnelImage, id)
	if result.Error != nil {
		return result.Error
	}
	return nil
}
