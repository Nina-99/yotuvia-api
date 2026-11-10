package models

import (
	"time"

	"gorm.io/gorm"
)

// BoostCampaign is a prepaid CPM promotion of one feed post.
//
// MONEY: the budget is debited UP FRONT from the center's token account when the
// campaign is created (`TokenAccountService.Consume`), so the platform never
// extends credit for ads and a run-away campaign can never overspend. Every
// billable impression then decrements the held budget; when `SpentMilli`
// reaches `BudgetMilli` the campaign is exhausted and drops out of the auction.
//
// Amounts are stored in MILLI-tokens (1 token = 1000 milli), matching the token
// ledger, so per-impression costs (a fraction of a CPM unit) stay exact.
type BoostCampaign struct {
	gorm.Model
	BeautyCenterID uint `gorm:"index;not null" json:"beauty_center_id"`
	FeedPostID     uint `gorm:"index;not null" json:"feed_post_id"`

	// BudgetMilli is the prepaid budget; SpentMilli is what has been settled.
	BudgetMilli int64 `gorm:"not null" json:"budget_milli"`
	SpentMilli  int64 `gorm:"not null;default:0" json:"spent_milli"`

	// ReservedMilli is how much this campaign holds on the token account at
	// creation. Legacy campaigns debited the budget up front and carry 0.
	// ReleasedMilli tracks how much of the hold has been returned, so pausing
	// twice never releases the same funds twice.
	ReservedMilli int64 `gorm:"not null;default:0" json:"reserved_milli"`
	ReleasedMilli int64 `gorm:"not null;default:0" json:"released_milli"`

	// AccountingModel states how the budget was funded. New rows are "reserve"
	// (fresh code sets it explicitly); the column default keeps every pre-existing
	// campaign on "legacy_debit", where the budget was debited at creation.
	AccountingModel string `gorm:"size:20;not null;default:'legacy_debit'" json:"accounting_model"`

	// BidCPMMilli is the maximum the center will pay per 1000 impressions. The
	// auction charges the SECOND price (see SelectBoostWinners), never above it.
	BidCPMMilli int64 `gorm:"not null" json:"bid_cpm_milli"`

	Impressions int64 `gorm:"not null;default:0" json:"impressions"`

	// Status: active | paused | exhausted | ended. Exhausted is derived from the
	// budget; paused is a manual switch; ended is a campaign past ends_at whose
	// remainder was released; active is the only billable state.
	Status string `gorm:"size:16;not null;default:'active'" json:"status"`

	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`

	FeedPost FeedPost `gorm:"foreignKey:FeedPostID" json:"-"`
}
