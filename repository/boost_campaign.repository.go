package repository

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
)

// BoostCampaignRepository persists CPM boost campaigns.
type BoostCampaignRepository interface {
	Create(campaign *models.BoostCampaign) error
	Update(campaign *models.BoostCampaign) error
	Delete(id uint) error
	// FindByID returns (nil, nil) when the campaign does not exist, so callers
	// can map "missing" to a 404 without treating it as a DB failure.
	FindByID(id uint) (*models.BoostCampaign, error)
	FindByCenter(centerID uint) ([]models.BoostCampaign, error)
	// FindEligible returns ACTIVE campaigns inside their window that still have
	// budget, highest bid first, capped at `limit` (the auction scan bound).
	FindEligible(now time.Time, limit int) ([]models.BoostCampaign, error)
}
