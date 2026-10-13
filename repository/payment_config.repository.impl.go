package repository

import (
	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type PaymentConfigRepositoryImpl struct {
	db *gorm.DB
}

func NewPaymentConfigRepositoryImpl(db *gorm.DB) *PaymentConfigRepositoryImpl {
	return &PaymentConfigRepositoryImpl{db: db}
}

// GetByCenter returns the payment config owned by centerID, creating an empty
// row on first call. The previous implementation forced every read/write to the
// platform-wide singleton (ID = 1); the row is now keyed by beauty_center_id,
// which is what closes the cross-tenant hijack.
func (r *PaymentConfigRepositoryImpl) GetByCenter(centerID uint) (*models.PaymentConfig, error) {
	var config models.PaymentConfig
	err := r.db.Where("beauty_center_id = ?", centerID).First(&config).Error
	if err == gorm.ErrRecordNotFound {
		config = models.PaymentConfig{BeautyCenterID: centerID}
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

// Save persists the row. The row's BeautyCenterID is the storage key and is
// never rewritten here (the old Save forced ID = 1).
func (r *PaymentConfigRepositoryImpl) Save(config *models.PaymentConfig) error {
	return r.db.Save(config).Error
}
