package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type TokenAccountRepositoryImpl struct {
	db *gorm.DB
}

func NewTokenAccountRepositoryImpl(db *gorm.DB) TokenAccountRepository {
	return &TokenAccountRepositoryImpl{db: db}
}

func (r *TokenAccountRepositoryImpl) Create(account *models.TokenAccount) error {
	return r.db.Create(account).Error
}

func (r *TokenAccountRepositoryImpl) Update(account *models.TokenAccount) error {
	return r.db.Save(account).Error
}

func (r *TokenAccountRepositoryImpl) FindByBeautyCenterID(beautyCenterID uint) (*models.TokenAccount, error) {
	var account models.TokenAccount
	err := r.db.Where("beauty_center_id = ?", beautyCenterID).First(&account).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &account, nil
}
