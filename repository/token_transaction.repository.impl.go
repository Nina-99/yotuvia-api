package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type TokenTransactionRepositoryImpl struct {
	db *gorm.DB
}

func NewTokenTransactionRepositoryImpl(db *gorm.DB) TokenTransactionRepository {
	return &TokenTransactionRepositoryImpl{db: db}
}

func (r *TokenTransactionRepositoryImpl) Create(tx *models.TokenTransaction) error {
	return r.db.Create(tx).Error
}

func (r *TokenTransactionRepositoryImpl) FindByBeautyCenterID(beautyCenterID uint, page, limit int) ([]models.TokenTransaction, int64, error) {
	var txs []models.TokenTransaction
	var total int64
	r.db.Model(&models.TokenTransaction{}).Where("beauty_center_id = ?", beautyCenterID).Count(&total)
	offset := (page - 1) * limit
	err := r.db.Where("beauty_center_id = ?", beautyCenterID).Offset(offset).Limit(limit).Order("created_at DESC").Find(&txs).Error
	return txs, total, err
}
