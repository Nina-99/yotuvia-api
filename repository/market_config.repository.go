package repository

import (
	"github.com/Nina-99/yottu-api/models"
)

type MarketConfigRepository interface {
	Create(config *models.MarketConfig) error
	Update(config *models.MarketConfig) error
	// FindByCurrency returns the config row for a currency code, or nil when
	// absent — the service layer applies canonical defaults on nil.
	FindByCurrency(currency string) (*models.MarketConfig, error)
}
