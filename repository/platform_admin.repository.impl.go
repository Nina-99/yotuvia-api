package repository

import (
	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type PlatformAdminRepositoryImpl struct {
	db *gorm.DB
}

func NewPlatformAdminRepositoryImpl(db *gorm.DB) PlatformAdminRepository {
	return &PlatformAdminRepositoryImpl{db: db}
}

func (r *PlatformAdminRepositoryImpl) Create(admin *models.PlatformAdmin) error {
	return r.db.Create(admin).Error
}

func (r *PlatformAdminRepositoryImpl) FindByEmail(email string) (*models.PlatformAdmin, error) {
	var admin models.PlatformAdmin
	err := r.db.Where("email = ?", email).First(&admin).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &admin, nil
}
