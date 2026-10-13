package repository

import (
	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type SocialMediaRepositoryImpl struct {
	db *gorm.DB
}

func NewSocialMediaRepositoryImpl(db *gorm.DB) SocialMediaRepository {
	return &SocialMediaRepositoryImpl{db: db}
}

func (r *SocialMediaRepositoryImpl) Create(socialMedia *models.SocialMedia) error {
	return r.db.Create(socialMedia).Error
}

func (r *SocialMediaRepositoryImpl) Update(socialMedia *models.SocialMedia) error {
	return r.db.Save(socialMedia).Error
}

func (r *SocialMediaRepositoryImpl) Delete(socialMedia *models.SocialMedia) error {
	return r.db.Delete(socialMedia).Error
}

func (r *SocialMediaRepositoryImpl) FindByID(id uint) (*models.SocialMedia, error) {
	var socialMedia models.SocialMedia
	err := r.db.First(&socialMedia, id).Error
	if err != nil {
		return nil, err
	}
	return &socialMedia, nil
}

func (r *SocialMediaRepositoryImpl) FindByBeautyCenterID(beautyCenterID uint) (*models.SocialMedia, error) {
	var socialMedia models.SocialMedia
	err := r.db.Where("beauty_center_id = ?", beautyCenterID).First(&socialMedia).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &socialMedia, nil
}
