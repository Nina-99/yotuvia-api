package repository

import "github.com/Nina-99/yotuvia-api/models"

//go:generate mockery --name=PaymentConfigRepository --output=../mocks --outpkg=mocks
type PaymentConfigRepository interface {
	// GetByCenter returns the payment config owned by centerID, creating an
	// empty row on first access so a fresh centre always has editable defaults.
	GetByCenter(centerID uint) (*models.PaymentConfig, error)
	// Save persists the row keyed by its BeautyCenterID (the primary key is
	// filled by GetByCenter).
	Save(config *models.PaymentConfig) error
}
