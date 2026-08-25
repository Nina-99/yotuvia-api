package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type MarketConfigRepositoryImpl struct {
	db *gorm.DB
}

func NewMarketConfigRepositoryImpl(db *gorm.DB) MarketConfigRepository {
	return &MarketConfigRepositoryImpl{db: db}
}

func (r *MarketConfigRepositoryImpl) Create(config *models.MarketConfig) error {
	return r.db.Create(config).Error
}

func (r *MarketConfigRepositoryImpl) Update(config *models.MarketConfig) error {
	return r.db.Save(config).Error
}

func (r *MarketConfigRepositoryImpl) FindByCurrency(currency string) (*models.MarketConfig, error) {
	var config models.MarketConfig
	err := r.db.Where("currency = ?", currency).First(&config).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}
