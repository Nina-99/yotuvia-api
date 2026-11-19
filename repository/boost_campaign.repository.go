package repository

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

// BoostCampaignRepository persists CPM boost campaigns.
// EligibleBoost is an auction candidate together with its centre's coordinates.
// The join exists so the auction can drop campaigns the viewer could never act
// on (another city) without a second round trip per candidate.
// BoostAggregates is the platform-wide ad summary.
type BoostAggregates struct {
	Campaigns       int64
	ActiveCampaigns int64
	Impressions     int64
	SpentMilli      int64
	UnservedWins    int64
}

// BoostTopCampaign is one row of the platform's sponsor leaderboard.
type BoostTopCampaign struct {
	CampaignID   uint
	CenterID     uint
	CenterName   string
	SpentMilli   int64
	Impressions  int64
	UnservedWins int64
}

// BoostDailyRow is one day of a campaign's delivery.
type BoostDailyRow struct {
	Day         string // YYYY-MM-DD
	Impressions int64
	// SpendMilli is the day's billed cost, derived from each impression's signed
	// second price (price_cpm_milli / 1000 per view).
	SpendMilli int64
}

type EligibleBoost struct {
	models.BoostCampaign
	CenterLatitude  float64
	CenterLongitude float64
}

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
	FindEligible(now time.Time, limit int) ([]EligibleBoost, error)

	// PlatformAggregates returns the platform-wide ad totals: how many campaigns
	// exist and how many are running, plus the sum of everything billed.
	PlatformAggregates() (BoostAggregates, error)

	// DailyAllImpressions is the platform-wide version of DailyImpressions.
	DailyAllImpressions() ([]BoostDailyRow, error)

	// TopBySpend lists the campaigns that consumed the most budget, with the
	// centre that bought them (the platform needs WHO is advertising, not just
	// the totals).
	TopBySpend(limit int) ([]BoostTopCampaign, error)

	// DailyImpressions returns the campaign's billed views grouped by calendar
	// day, oldest first, for the owner's performance chart.
	DailyImpressions(campaignID uint) ([]BoostDailyRow, error)

	// AddUnservedWins increments the delivery-miss counter for each campaign.
	AddUnservedWins(campaignIDs []uint) error

	// CountImpressionsByViewer returns, per campaign, how many BILLED views this
	// viewer has already produced. It powers the frequency cap: the advertiser
	// pays per qualified view, so this is exactly the number that must stop
	// growing for one person.
	CountImpressionsByViewer(viewerID uint, campaignIDs []uint) (map[uint]int64, error)
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
