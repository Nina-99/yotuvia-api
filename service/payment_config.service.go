package service

import "github.com/Nina-99/yotuvia-api/models"

type PaymentConfigService interface {
	// Get returns the payment config owned by centerID (empty defaults for a
	// centre that has none yet). The centre is always explicit; there is no
	// platform-wide fallback.
	Get(centerID uint) (*models.PaymentConfig, error)
	// GetTiers returns the parsed per-currency bonus ladder of centerID.
	GetTiers(centerID uint) (models.PaymentConfigTiers, error)
	// Update replaces the non-empty fields of centerID's config. A nil tiers
	// map leaves the ladder untouched (used by the QR-only upload path).
	Update(centerID uint, qrURL, phone string, tiers models.PaymentConfigTiers) error
}
