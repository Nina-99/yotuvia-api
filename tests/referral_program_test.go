package tests

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func referralSUT(t *testing.T) (service.ReferralService, *gorm.DB, repository.TokenAccountRepository) {
	t.Helper()
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	referralRepo := repository.NewReferralRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	return service.NewReferralServiceImpl(referralRepo, centerRepo, nil, accountRepo), db, accountRepo
}

func seedReferralAccounts(t *testing.T, db *gorm.DB, centerIDs ...uint) {
	t.Helper()
	for _, id := range centerIDs {
		seedDashboardCenter(t, db, id, fmt.Sprintf("Center %d", id), fmt.Sprintf("c%d@test.com", id), true)
		require.NoError(t, db.Create(&models.TokenAccount{
			BeautyCenterID: id, Balance: 0, Currency: "BOB",
		}).Error)
	}
}

func balanceOf(t *testing.T, repo repository.TokenAccountRepository, centerID uint) int64 {
	t.Helper()
	account, err := repo.FindByBeautyCenterID(centerID)
	require.NoError(t, err)
	require.NotNil(t, account)
	return account.Balance
}

func referralLedgerRows(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&models.TokenTransaction{}).Where("type = ?", "referral").Count(&n).Error)
	return n
}

// TestReferral_EntryDoesNotMoveMoney — entering a code only records a pending
// referral; the 20/10 rewards are not credited yet.
func TestReferral_EntryDoesNotMoveMoney(t *testing.T) {
	svc, db, accountRepo := referralSUT(t)
	seedReferralAccounts(t, db, 1, 2)

	require.NoError(t, svc.CreateReferralByCode(2, "REF001"))

	var referral models.Referral
	require.NoError(t, db.First(&referral).Error)
	assert.Equal(t, "pending", referral.Status)
	assert.Equal(t, int64(20_000), referral.RewardReferrer)
	assert.Equal(t, int64(10_000), referral.RewardReferred)
	assert.Equal(t, int64(0), balanceOf(t, accountRepo, 1))
	assert.Equal(t, int64(0), balanceOf(t, accountRepo, 2))
	assert.Equal(t, int64(0), referralLedgerRows(t, db))
}

// TestReferral_MonthlyLimitIsACalendarWindow — 10 referrals this month block the
// 11th, while referrals from a previous month do not count.
func TestReferral_MonthlyLimitIsACalendarWindow(t *testing.T) {
	svc, db, _ := referralSUT(t)
	seedReferralAccounts(t, db, 1)
	for id := uint(2); id <= 12; id++ {
		seedDashboardCenter(t, db, id, fmt.Sprintf("Center %d", id), fmt.Sprintf("c%d@test.com", id), true)
	}

	for id := uint(2); id <= 11; id++ {
		require.NoError(t, db.Create(&models.Referral{
			ReferrerID: 1, ReferredID: id, Status: "pending",
		}).Error)
	}

	err := svc.CreateReferralByCode(12, "REF001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "monthly referral limit")

	// Move last month's referrals out of the window: the 12th center can enter.
	lastMonth := time.Now().UTC().AddDate(0, -1, 0)
	require.NoError(t, db.Model(&models.Referral{}).
		Where("referrer_id = ?", 1).
		Update("created_at", lastMonth).Error)

	require.NoError(t, svc.CreateReferralByCode(12, "REF001"))
}

// TestReferral_VerificationCreditsBothWalletsOnce — processing the reward
// credits 20/10 exactly once and flips the referral to credited.
func TestReferral_VerificationCreditsBothWalletsOnce(t *testing.T) {
	svc, db, accountRepo := referralSUT(t)
	seedReferralAccounts(t, db, 1, 2)
	require.NoError(t, svc.CreateReferralByCode(2, "REF001"))

	require.NoError(t, svc.ProcessReferralReward(2))

	assert.Equal(t, int64(20_000), balanceOf(t, accountRepo, 1))
	assert.Equal(t, int64(10_000), balanceOf(t, accountRepo, 2))
	assert.Equal(t, int64(2), referralLedgerRows(t, db))

	var referral models.Referral
	require.NoError(t, db.First(&referral).Error)
	assert.Equal(t, "credited", referral.Status)
	assert.NotNil(t, referral.CreditedAt)

	// Idempotent: a second pass changes nothing.
	require.NoError(t, svc.ProcessReferralReward(2))
	assert.Equal(t, int64(20_000), balanceOf(t, accountRepo, 1))
	assert.Equal(t, int64(2), referralLedgerRows(t, db))
}

// TestReferral_FailedCreditLeavesPending — if the ledger write fails, the whole
// transaction rolls back: no partial credit and the referral stays pending.
func TestReferral_FailedCreditLeavesPending(t *testing.T) {
	svc, db, accountRepo := referralSUT(t)
	seedReferralAccounts(t, db, 1, 2)
	require.NoError(t, svc.CreateReferralByCode(2, "REF001"))

	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(
		"fail_referral_ledger", func(d *gorm.DB) {
			if _, ok := d.Statement.Model.(*models.TokenTransaction); ok {
				d.AddError(errors.New("injected ledger failure"))
			}
		},
	))

	err := svc.ProcessReferralReward(2)
	require.Error(t, err)

	var referral models.Referral
	require.NoError(t, db.First(&referral).Error)
	assert.Equal(t, "pending", referral.Status, "rollback must keep the referral pending")
	assert.Equal(t, int64(0), balanceOf(t, accountRepo, 1))
	assert.Equal(t, int64(0), balanceOf(t, accountRepo, 2))
	assert.Equal(t, int64(0), referralLedgerRows(t, db))
}

// TestBeautyCenter_MarkVerifiedCreditsReferral — the verification transition is
// the trigger that credits the referral (service wiring, nil-safe).
func TestBeautyCenter_MarkVerifiedCreditsReferral(t *testing.T) {
	svc, db, accountRepo := referralSUT(t)
	seedReferralAccounts(t, db, 1, 2)
	completeCenterProfile(t, db, 2)
	require.NoError(t, svc.CreateReferralByCode(2, "REF001"))

	centerSvc := service.NewBeautyCenterServiceImpl(
		repository.NewBeautyCenterRepositoryImpl(db),
		nil, // mission engine not needed here
		validator.New(),
		nil,
		service.BeautyCenterDeps{ReferralService: svc},
	)
	require.NoError(t, centerSvc.MarkVerified(2))

	assert.Equal(t, int64(20_000), balanceOf(t, accountRepo, 1))
	assert.Equal(t, int64(10_000), balanceOf(t, accountRepo, 2))

	var referral models.Referral
	require.NoError(t, db.First(&referral).Error)
	assert.Equal(t, "credited", referral.Status)
}
