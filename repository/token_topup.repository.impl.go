package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type TokenTopUpRepositoryImpl struct {
	db *gorm.DB
}

func NewTokenTopUpRepositoryImpl(db *gorm.DB) TokenTopUpRepository {
	return &TokenTopUpRepositoryImpl{db: db}
}

func (r *TokenTopUpRepositoryImpl) Create(topup *models.TokenTopUp) error {
	return r.db.Create(topup).Error
}

func (r *TokenTopUpRepositoryImpl) Update(topup *models.TokenTopUp) error {
	return r.db.Save(topup).Error
}

func (r *TokenTopUpRepositoryImpl) FindByID(id uint) (*models.TokenTopUp, error) {
	var topup models.TokenTopUp
	err := r.db.First(&topup, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &topup, nil
}

func (r *TokenTopUpRepositoryImpl) FindByBeautyCenterID(beautyCenterID uint) ([]models.TokenTopUp, error) {
	var topups []models.TokenTopUp
	err := r.db.Where("beauty_center_id = ?", beautyCenterID).Order("created_at DESC").Find(&topups).Error
	return topups, err
}

func (r *TokenTopUpRepositoryImpl) FindPending() ([]models.TokenTopUp, error) {
	var topups []models.TokenTopUp
	err := r.db.Where("status = ?", "pending").Order("created_at ASC").Find(&topups).Error
	return topups, err
}

func (r *TokenTopUpRepositoryImpl) FindAll(page, limit int) ([]models.TokenTopUp, int64, error) {
	var topups []models.TokenTopUp
	var total int64
	r.db.Model(&models.TokenTopUp{}).Count(&total)
	offset := (page - 1) * limit
	err := r.db.Offset(offset).Limit(limit).Order("created_at DESC").Find(&topups).Error
	return topups, total, err
}
