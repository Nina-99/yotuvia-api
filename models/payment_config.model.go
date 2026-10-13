package models

import (
	"time"

	"gorm.io/gorm"
)

// PaymentConfig holds ONE beauty centre's token-payment configuration: the bank
// QR image, reference phone number, and per-currency tier definitions. It is
// keyed by BeautyCenterID — a centre admin may only read/write its own row, and
// the public read resolves an explicit centre so no centre ever serves another
// centre's QR.
//
// The column carries `default:0` so AutoMigrate can add it to a live table
// without tripping PostgreSQL's "NOT NULL column contains null values" rule.
// The pre-multi-tenant singleton row is preserved with BeautyCenterID = 0 for
// rollback: it is never served (public reads reject ids <= 0) and the startup
// backfill copies its values to every existing centre exactly once.
type PaymentConfig struct {
	ID             uint           `gorm:"primarykey" json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	BeautyCenterID uint           `gorm:"uniqueIndex;default:0" json:"beauty_center_id"`
	PaymentQRURL   string         `gorm:"type:text" json:"payment_qr_url"`
	PaymentPhone   string         `gorm:"size:30" json:"payment_phone"`
	TiersJSON      string         `gorm:"type:text" json:"tiers_json"`
}

// PaymentConfigTier represents a single bonus tier for a given currency.
type PaymentConfigTier struct {
	Min      float64 `json:"min"`
	BonusPct float64 `json:"bonusPct"`
}

// PaymentConfigTiers is the JSON structure stored in TiersJSON.
// Keys are ISO currency codes (e.g. "USD", "VES").
type PaymentConfigTiers map[string][]PaymentConfigTier
