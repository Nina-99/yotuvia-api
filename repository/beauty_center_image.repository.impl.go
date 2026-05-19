package repository

import (
	"errors"

	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type BeautyCenterImageRepositoryImpl struct {
	Db *gorm.DB
}

func NewBeautyCenterImageRepositoryImpl(Db *gorm.DB) BeautyCenterImageRepository {
	return &BeautyCenterImageRepositoryImpl{Db: Db}
}

func (r *BeautyCenterImageRepositoryImpl) Save(beautyCenterImage *models.BeautyCenterImage) error {
	result := r.Db.Save(beautyCenterImage)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (r *BeautyCenterImageRepositoryImpl) FindByID(id uint) (*models.BeautyCenterImage, error) {
	var beautyCenterImage models.BeautyCenterImage
	result := r.Db.First(&beautyCenterImage, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("beautyCenter image not found")
		}
		return nil, result.Error
	}
	return &beautyCenterImage, nil
}

func (r *BeautyCenterImageRepositoryImpl) FindByBeautyCenterID(beautyCenterID uint) ([]models.BeautyCenterImage, error) {
	var beautyCenterImage []models.BeautyCenterImage
	result := r.Db.Where("beauty_center_id = ?", beautyCenterID).Find(&beautyCenterImage)
	if result.Error != nil {
		return nil, result.Error
	}
	return beautyCenterImage, nil
}

func (r *BeautyCenterImageRepositoryImpl) Delete(id uint) error {
	var beautyCenterImage models.BeautyCenterImage
	result := r.Db.Delete(&beautyCenterImage, id)
	if result.Error != nil {
		return result.Error
	}
	return nil
}
