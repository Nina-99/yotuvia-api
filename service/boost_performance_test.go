package service

// The campaign report an owner uses to decide whether to renew. Every assertion
// here is about MONEY being reported honestly: the effective CPM must come from
// what was actually charged (second price), not from the bid.

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoostPerformance_ReportsWhatWasActuallyBilled(t *testing.T) {
	svc, db, post := newAuctionSvc(t)
	now := time.Now()
	campaign, err := svc.CreateCampaign(1, post, utils.ToMilli(50), 800, now, now.Add(time.Hour))
	require.NoError(t, err)

	// Four delivered views at the signed second price of 250 milli CPM each:
	// 4 views * 250/1000 = 1 milli per view -> 4 milli total.
	for i := 0; i < 4; i++ {
		require.NoError(t, db.Create(&models.BoostImpression{
			CampaignID: campaign.ID, ViewerID: uint(10 + i),
			Nonce: "nonce-" + string(rune('a'+i)), PriceCPMMilli: 250,
			CreatedAt: now,
		}).Error)
	}
	// One settled batch of those four, so the campaign row carries the spend.
	require.NoError(t, svc.SettleImpressions(campaign.ID, 4, 250))

	// Two auction wins that never rendered.
	require.NoError(t, svc.RecordUnservedWins([]uint{campaign.ID}))
	require.NoError(t, svc.RecordUnservedWins([]uint{campaign.ID}))

	perf, err := svc.Performance(1, campaign.ID)
	require.NoError(t, err)

	assert.Equal(t, float64(0.004), perf.Spent, "4 views at 250 milli CPM cost 4 milli = 0.004 Ytv")
	assert.Equal(t, int64(4), perf.Impressions)
	assert.Equal(t, int64(2), perf.UnservedWins)
	assert.InDelta(t, 1.0, perf.EffectiveCPM, 1e-9,
		"effective CPM must be spend/impressions*1000 = 0.004/4*1000 = 1 Ytv per 1000")
	// 4 delivered of 6 wins.
	assert.InDelta(t, 4.0/6.0, perf.DeliveryRate, 1e-9)
	assert.InDelta(t, 49.996, perf.Remaining, 1e-6)
	require.Len(t, perf.Daily, 1, "one calendar day of delivery")
	assert.Equal(t, int64(4), perf.Daily[0].Impressions)
}

func TestBoostPerformance_EmptyCampaign(t *testing.T) {
	svc, _, post := newAuctionSvc(t)
	now := time.Now()
	campaign, err := svc.CreateCampaign(1, post, utils.ToMilli(50), 800, now, now.Add(time.Hour))
	require.NoError(t, err)

	perf, err := svc.Performance(1, campaign.ID)
	require.NoError(t, err)

	assert.Zero(t, perf.EffectiveCPM, "no impressions -> no effective CPM (never NaN)")
	assert.Zero(t, perf.DeliveryRate, "no auction wins -> no delivery rate (never NaN)")
	assert.Empty(t, perf.Daily)
	assert.Equal(t, float64(50), perf.Budget)
	assert.Zero(t, perf.Spent)
}

func TestBoostPerformance_IsOwnerScoped(t *testing.T) {
	svc, db, post := newAuctionSvc(t)
	createTestCenter(db, 2)
	now := time.Now()
	campaign, err := svc.CreateCampaign(1, post, utils.ToMilli(50), 800, now, now.Add(time.Hour))
	require.NoError(t, err)

	_, err = svc.Performance(2, campaign.ID)
	assert.ErrorIs(t, err, apperrors.ErrForbidden,
		"another centre must not read this campaign's report")

	_, err = svc.Performance(1, 999_999)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

// TestBoostOverview_AggregatesTheWholeBook — the platform's revenue view: the
// CPM must come from money actually charged, and the leaderboard must name the
// advertiser.
func TestBoostOverview_AggregatesTheWholeBook(t *testing.T) {
	svc, db, post := newAuctionSvc(t)
	now := time.Now()
	campaign, err := svc.CreateCampaign(1, post, utils.ToMilli(50), 800, now, now.Add(time.Hour))
	require.NoError(t, err)

	for i := 0; i < 4; i++ {
		require.NoError(t, db.Create(&models.BoostImpression{
			CampaignID: campaign.ID, ViewerID: uint(20 + i),
			Nonce: "ov-" + string(rune('a'+i)), PriceCPMMilli: 250, CreatedAt: now,
		}).Error)
	}
	require.NoError(t, svc.SettleImpressions(campaign.ID, 4, 250))
	require.NoError(t, svc.RecordUnservedWins([]uint{campaign.ID}))

	overview, err := svc.PlatformOverview()
	require.NoError(t, err)

	assert.Equal(t, int64(1), overview.Campaigns)
	assert.Equal(t, int64(1), overview.ActiveCampaigns)
	assert.Equal(t, int64(4), overview.Impressions)
	assert.Equal(t, int64(1), overview.UnservedWins)
	assert.InDelta(t, 0.004, overview.Spent, 1e-9)
	assert.InDelta(t, 1.0, overview.EffectiveCPM, 1e-9)
	assert.InDelta(t, 0.8, overview.DeliveryRate, 1e-9) // 4 of 5 wins
	require.NotEmpty(t, overview.Daily)

	require.Len(t, overview.TopCampaigns, 1)
	assert.Equal(t, campaign.ID, overview.TopCampaigns[0].CampaignID)
	assert.Equal(t, "Test Center", overview.TopCampaigns[0].CenterName,
		"the platform must know WHO is advertising")
	assert.InDelta(t, 0.004, overview.TopCampaigns[0].Spent, 1e-9)
}

// TestBoostOverview_EmptyBookNeverDividesByZero
func TestBoostOverview_EmptyBookNeverDividesByZero(t *testing.T) {
	svc, _, _ := newAuctionSvc(t)

	overview, err := svc.PlatformOverview()
	require.NoError(t, err)

	assert.Zero(t, overview.EffectiveCPM)
	assert.Zero(t, overview.DeliveryRate)
	assert.Empty(t, overview.TopCampaigns)
	assert.Empty(t, overview.Daily)
}
