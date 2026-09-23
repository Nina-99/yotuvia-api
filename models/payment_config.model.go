package models

import (
	"time"

	"gorm.io/gorm"
)

// PaymentConfig is a singleton table storing the platform-wide payment
// configuration: the bank QR image, reference phone number, and
// per-currency tier definitions. Only one row ever exists (ID = 1).
type PaymentConfig struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	PaymentQRURL string         `gorm:"type:text" json:"payment_qr_url"`
	PaymentPhone string         `gorm:"size:30" json:"payment_phone"`
	TiersJSON    string         `gorm:"type:text" json:"tiers_json"`
}

// PaymentConfigTier represents a single bonus tier for a given currency.
type PaymentConfigTier struct {
	Min      float64 `json:"min"`
	BonusPct float64 `json:"bonusPct"`
}

// PaymentConfigTiers is the JSON structure stored in TiersJSON.
// Keys are ISO currency codes (e.g. "USD", "VES").
type PaymentConfigTiers map[string][]PaymentConfigTier
