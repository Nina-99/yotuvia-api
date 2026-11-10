package repository

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

// BoostCampaignRepository persists CPM boost campaigns.
type BoostCampaignRepository interface {
	Create(campaign *models.BoostCampaign) error
	Update(campaign *models.BoostCampaign) error
	// UpdateInTx persists a campaign inside a CALLER-managed transaction so a
	// release or settlement commits atomically with the token-ledger write.
	UpdateInTx(tx *gorm.DB, campaign *models.BoostCampaign) error
	Delete(id uint) error
	// FindByID returns (nil, nil) when the campaign does not exist, so callers
	// can map "missing" to a 404 without treating it as a DB failure.
	FindByID(id uint) (*models.BoostCampaign, error)
	FindByCenter(centerID uint) ([]models.BoostCampaign, error)
	// FindEligible returns ACTIVE campaigns inside their window that still have
	// budget, highest bid first, capped at `limit` (the auction scan bound).
	FindEligible(now time.Time, limit int) ([]models.BoostCampaign, error)
	// FindExpired returns active/paused campaigns whose window already closed,
	// so the service can release their remaining hold.
	FindExpired(now time.Time) ([]models.BoostCampaign, error)
	// AddSpendInTx books impressions against the budget inside a CALLER-managed
	// transaction. It is a conditional UPDATE, so concurrent settlements can
	// never exceed the budget. The boolean reports whether the spend fit.
	AddSpendInTx(tx *gorm.DB, campaignID uint, impressions, costMilli int64) (bool, error)
	// MarkExhaustedInTx flips an active campaign to exhausted once its spend
	// reached the budget.
	MarkExhaustedInTx(tx *gorm.DB, campaignID uint) error
	// CreateImpressionInTx inserts the billing dedupe row. It returns false when
	// the (campaign, viewer, nonce) triple already exists, so a replayed report
	// is a no-op instead of a second charge.
	CreateImpressionInTx(tx *gorm.DB, impression *models.BoostImpression) (bool, error)
}
