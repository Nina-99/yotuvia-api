package service

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func newImpressionTestService(t *testing.T) (BoostService, *gorm.DB, uint) {
	t.Helper()
	db := setupBoostTestDB(t)
	createTestCenter(db, 1)
	tokenSvc := fundCenter(t, db, 1, 100)
	postID := createFeedPost(t, db, 1)
	svc := NewBoostServiceImpl(
		repository.NewBoostCampaignRepositoryImpl(db),
		repository.NewFeedPostRepositoryImpl(db),
		tokenSvc,
		repository.NewTokenAccountRepositoryImpl(db),
		nil,
	)
	return svc, db, postID
}

func storedCampaign(t *testing.T, db *gorm.DB, id uint) models.BoostCampaign {
	t.Helper()
	var c models.BoostCampaign
	assert.NoError(t, db.First(&c, id).Error)
	return c
}

func impressionCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	assert.NoError(t, db.Model(&models.BoostImpression{}).Count(&n).Error)
	return n
}

// TestBoostImpression_QualifiedViewBillsOnce — a qualified view charges one
// impression; replaying the same token never charges twice.
func TestBoostImpression_QualifiedViewBillsOnce(t *testing.T) {
	svc, db, postID := newImpressionTestService(t)
	now := time.Now()
	campaign, err := svc.CreateCampaign(1, postID, utils.ToMilli(10), 1000, now, now.Add(time.Hour))
	assert.NoError(t, err)

	token, err := svc.ImpressionToken(campaign.ID, 1000)
	assert.NoError(t, err)

	assert.NoError(t, svc.SettleImpression(7, token, 60, false))
	stored := storedCampaign(t, db, campaign.ID)
	assert.Equal(t, int64(1), stored.SpentMilli, "one impression at 1 milli")
	assert.Equal(t, int64(1), stored.Impressions)
	assert.Equal(t, int64(1), impressionCount(t, db))

	// Replayed report: accepted as a no-op, never billed twice.
	assert.NoError(t, svc.SettleImpression(7, token, 60, false))
	stored = storedCampaign(t, db, campaign.ID)
	assert.Equal(t, int64(1), stored.SpentMilli)
	assert.Equal(t, int64(1), impressionCount(t, db))
}

// TestBoostImpression_UnqualifiedViewIsFree — a view below the threshold is not
// billed and does not consume the dedupe slot.
func TestBoostImpression_UnqualifiedViewIsFree(t *testing.T) {
	svc, db, postID := newImpressionTestService(t)
	now := time.Now()
	campaign, err := svc.CreateCampaign(1, postID, utils.ToMilli(10), 1000, now, now.Add(time.Hour))
	assert.NoError(t, err)

	token, err := svc.ImpressionToken(campaign.ID, 1000)
	assert.NoError(t, err)

	assert.NoError(t, svc.SettleImpression(9, token, 10, false))
	stored := storedCampaign(t, db, campaign.ID)
	assert.Equal(t, int64(0), stored.SpentMilli)
	assert.Equal(t, int64(0), impressionCount(t, db))
}

// TestBoostImpression_TamperedTokenRejected — changing the signed payload must
// be detected.
func TestBoostImpression_TamperedTokenRejected(t *testing.T) {
	svc, _, postID := newImpressionTestService(t)
	now := time.Now()
	campaign, err := svc.CreateCampaign(1, postID, utils.ToMilli(10), 1000, now, now.Add(time.Hour))
	assert.NoError(t, err)

	token, err := svc.ImpressionToken(campaign.ID, 1000)
	assert.NoError(t, err)

	err = svc.SettleImpression(7, token+"x", 100, true)
	assert.ErrorIs(t, err, ErrInvalidImpressionToken)
}

// TestBoostImpression_ExpiredTokenRejected — a token past its billing window is
// refused.
func TestBoostImpression_ExpiredTokenRejected(t *testing.T) {
	svc, _, postID := newImpressionTestService(t)
	now := time.Now()
	campaign, err := svc.CreateCampaign(1, postID, utils.ToMilli(10), 1000, now, now.Add(time.Hour))
	assert.NoError(t, err)

	impl := svc.(*BoostServiceImpl)
	token, err := SignImpressionToken(impl.impressionSecret, ImpressionPayload{
		CampaignID:    campaign.ID,
		PriceCPMMilli: 1000,
		Nonce:         "expired-nonce",
		Exp:           time.Now().Add(-time.Minute).Unix(),
	})
	assert.NoError(t, err)

	err = svc.SettleImpression(7, token, 100, true)
	assert.ErrorIs(t, err, ErrExpiredImpressionToken)
}

// TestBoostService_LegacyPauseReleasesNothing — a pre-retention campaign has no
// hold, so pausing it must not touch the wallet.
func TestBoostService_LegacyPauseReleasesNothing(t *testing.T) {
	svc, db, _ := newImpressionTestService(t)

	legacy := models.BoostCampaign{
		BeautyCenterID:  1,
		FeedPostID:      1,
		BudgetMilli:     utils.ToMilli(10),
		SpentMilli:      utils.ToMilli(2),
		BidCPMMilli:     1000,
		Status:          BoostStatusActive,
		AccountingModel: BoostAccountingLegacy,
		StartsAt:        time.Now().Add(-time.Hour),
		EndsAt:          time.Now().Add(time.Hour),
	}
	assert.NoError(t, db.Create(&legacy).Error)

	var before models.TokenAccount
	assert.NoError(t, db.Where("beauty_center_id = ?", 1).First(&before).Error)

	assert.NoError(t, svc.PauseCampaign(1, legacy.ID))

	var after models.TokenAccount
	assert.NoError(t, db.Where("beauty_center_id = ?", 1).First(&after).Error)
	assert.Equal(t, before.Balance, after.Balance, "legacy pause must not move money")
	assert.Equal(t, int64(0), after.ReservedMilli, "legacy pause must not release a hold")

	stored := storedCampaign(t, db, legacy.ID)
	assert.Equal(t, BoostStatusPaused, stored.Status)
	assert.Equal(t, int64(0), stored.ReleasedMilli)
}

// TestBoostImpression_LegacyCampaignBillsWithoutHold — a campaign created before
// retention (ReservedMilli 0) still bills its campaign accounting and never
// touches the token account.
func TestBoostImpression_LegacyCampaignBillsWithoutHold(t *testing.T) {
	svc, db, _ := newImpressionTestService(t)
	legacy := models.BoostCampaign{
		BeautyCenterID:  1,
		FeedPostID:      1,
		BudgetMilli:     utils.ToMilli(10),
		BidCPMMilli:     1000,
		Status:          BoostStatusActive,
		AccountingModel: BoostAccountingLegacy,
		StartsAt:        time.Now().Add(-time.Hour),
		EndsAt:          time.Now().Add(time.Hour),
	}
	assert.NoError(t, db.Create(&legacy).Error)

	token, err := svc.ImpressionToken(legacy.ID, 1000)
	assert.NoError(t, err)
	assert.NoError(t, svc.SettleImpression(7, token, 100, true))

	stored := storedCampaign(t, db, legacy.ID)
	assert.Equal(t, int64(1), stored.SpentMilli)

	var conversions int64
	assert.NoError(t, db.Model(&models.TokenTransaction{}).Where("type = ?", "boost_spend").Count(&conversions).Error)
	assert.Equal(t, int64(0), conversions, "legacy campaigns have no hold to settle")
}
