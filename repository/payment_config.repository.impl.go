package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

const paymentConfigSingletonID uint = 1

type PaymentConfigRepositoryImpl struct {
	db *gorm.DB
}

func NewPaymentConfigRepositoryImpl(db *gorm.DB) *PaymentConfigRepositoryImpl {
	return &PaymentConfigRepositoryImpl{db: db}
}

// Get returns the singleton PaymentConfig row, creating it on first call.
func (r *PaymentConfigRepositoryImpl) Get() (*models.PaymentConfig, error) {
	var config models.PaymentConfig
	err := r.db.First(&config, paymentConfigSingletonID).Error
	if err == gorm.ErrRecordNotFound {
		config = models.PaymentConfig{ID: paymentConfigSingletonID}
		if err := r.db.Create(&config).Error; err != nil {
			return nil, err
		}
		return &config, nil
	}
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// Save persists the singleton PaymentConfig row.
func (r *PaymentConfigRepositoryImpl) Save(config *models.PaymentConfig) error {
	config.ID = paymentConfigSingletonID
	return r.db.Save(config).Error
}
