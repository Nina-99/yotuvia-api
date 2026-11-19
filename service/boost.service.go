package service

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
)

// BoostService owns the paid-promotion lifecycle: it holds the prepaid budget,
// settles impressions against it, and runs the auction that picks promoted
// slots for a feed page.
type BoostService interface {
	// CreateCampaign validates ownership, holds `budgetMilli` from the center's
	// token account up front and creates an ACTIVE campaign.
	CreateCampaign(centerID, feedPostID uint, budgetMilli, bidCPMMilli int64, startsAt, endsAt time.Time) (*models.BoostCampaign, error)
	// PauseCampaign flips an owned campaign to `paused` (it stops winning slots).
	PauseCampaign(centerID, campaignID uint) error
	// SettleImpressions books delivered impressions against the campaign's prepaid
	// hold at the auction price; the campaign is marked `exhausted` once spent
	// reaches budget.
	SettleImpressions(campaignID uint, impressions, priceCPMMilli int64) error
	// SweepEnded releases the remaining hold of campaigns past their window,
	// marks them ended and returns how many campaigns it closed.
	SweepEnded(now time.Time) (int, error)
	// ImpressionToken signs the one-view billing token for a served promoted
	// slot. The price is inside the signature.
	ImpressionToken(campaignID uint, priceCPMMilli int64) (string, error)
	// SettleImpression verifies a signed token and bills ONE qualified view.
	// Unqualified views, duplicate reports, expired tokens and exhausted
	// campaigns are no-ops.
	SettleImpression(viewerID uint, token string, watchPercentage float64, completed bool) error
	// PlatformOverview is the super_admin's view of the whole ad book: totals,
	// platform-wide effective CPM and delivery, the daily curve and the biggest
	// advertisers. It is the number that says whether ads are a business.
	PlatformOverview() (BoostOverview, error)

	// Performance builds the per-campaign report an owner needs to decide
	// whether to renew: delivered views, effective CPM, delivery rate and the
	// daily curve. It is owner-scoped by centerID.
	Performance(centerID, campaignID uint) (BoostPerformance, error)

	// ListByCenter returns the center's own campaigns (newest first).
	ListByCenter(centerID uint) ([]models.BoostCampaign, error)
	// SelectWinners runs the second-price auction over eligible campaigns for
	// `slots` promoted positions.
	// SelectWinners runs the second-price auction for the promoted slots.
	//
	// `viewer` is optional: with real coordinates, a campaign whose centre is
	// farther than BOOST_MAX_RADIUS_KM is not even auctioned — an ad in another
	// city spends the advertiser's budget on impressions it can never convert.
	// Without a position, the auction is the platform-wide one it always was.
	// viewerID enables the frequency cap: a campaign the viewer already produced
	// BOOST_FREQUENCY_CAP billed views for is not auctioned to them again.
	// RecordUnservedWins books, best-effort, the campaigns that won an auction
	// but could not be rendered. Never fails the feed.
	RecordUnservedWins(campaignIDs []uint) error

	SelectWinners(now time.Time, slots int, reserveCPMMilli int64, viewer *GeoPoint, viewerID uint) ([]BoostWinner, error)
}

// BoostPerformance is the campaign report. Money is in TOKENS (milli / 1000) to
// match every other owner-facing screen.
type BoostPerformance struct {
	CampaignID   uint
	Status       string
	Currency     string
	Budget       float64
	Spent        float64
	Remaining    float64
	Impressions  int64
	UnservedWins int64
	EffectiveCPM float64 // tokens per 1000 delivered views; 0 until one lands
	DeliveryRate float64 // delivered / (delivered + undelivered), 0..1
	StartsAt     time.Time
	EndsAt       time.Time
	Daily        []BoostDailyPoint
}

// BoostDailyPoint is one day of the curve.
type BoostDailyPoint struct {
	Day         string  `json:"day"`
	Impressions int64   `json:"impressions"`
	Spend       float64 `json:"spend"`
}

// BoostOverview is the platform-wide ad report.
type BoostOverview struct {
	Campaigns       int64
	ActiveCampaigns int64
	Impressions     int64
	Spent           float64
	UnservedWins    int64
	EffectiveCPM    float64
	DeliveryRate    float64
	TopCampaigns    []BoostTopCampaign
	Daily           []BoostDailyPoint
}

// BoostTopCampaign is one row of the sponsor leaderboard.
type BoostTopCampaign struct {
	CampaignID   uint    `json:"campaign_id"`
	CenterID     uint    `json:"center_id"`
	CenterName   string  `json:"center_name"`
	Spent        float64 `json:"spent"`
	Impressions  int64   `json:"impressions"`
	UnservedWins int64   `json:"unserved_wins"`
}
