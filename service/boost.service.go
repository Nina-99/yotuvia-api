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
	// ListByCenter returns the center's own campaigns (newest first).
	ListByCenter(centerID uint) ([]models.BoostCampaign, error)
	// SelectWinners runs the second-price auction over eligible campaigns for
	// `slots` promoted positions.
	SelectWinners(now time.Time, slots int, reserveCPMMilli int64) ([]BoostWinner, error)
}
