package tests

import (
	"context"
	"testing"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/stretchr/testify/assert"
)

// TestTokenLedger_ConsumeFailureReturnsErrorNoGraceRow — an atomic consumption
// that fails (insufficient balance) must return an error and MUST NOT create a
// "grace" transaction row (spec: "Failed write returns an error, not a grace
// row").
func TestTokenLedger_ConsumeFailureReturnsErrorNoGraceRow(t *testing.T) {
	db := newTokenLedgerDB(t)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	missionSvc := service.NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	notifSvc := service.NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), repository.NewPushDeviceRepositoryImpl(db))
	svc := service.NewTokenAccountServiceImpl(accountRepo, topUpRepo, centerRepo, missionSvc, notifSvc)

	// Deterministic overdraft floor: computeOverdraft grants the Premium tier
	// (-30 YOTTU = 30_000 milli) to any center with TotalPurchased >= 500_000
	// milli (500 YOTTU), independent of account age or top-up count. Seeding it
	// pins the floor instead of leaning on the "new center" magic default.
	//
	//   floor = balance + overdraft = 5_000 + 30_000 = 35_000 milli
	//
	// Consuming one milli past the floor must fail atomically (the earlier
	// 10_000 consume only reached the OLD no-overdraft floor of 5_000 and is no
	// longer an overdraw — that assertion was stale, not the code).
	acct := &models.TokenAccount{BeautyCenterID: 1, Balance: 5000, TotalPurchased: 500_000, Currency: "USD"}
	assert.NoError(t, accountRepo.Create(acct))

	err := svc.Consume(1, 35_001, 99, "overdraw attempt")
	assert.ErrorIs(t, err, apperrors.ErrInsufficientBalance)

	// No grace row and balance unchanged.
	var graceRows int64
	db.Model(&models.TokenTransaction{}).Where("beauty_center_id = ? AND type = ?", 1, "grace").Count(&graceRows)
	assert.Equal(t, int64(0), graceRows)

	got, err := accountRepo.FindByBeautyCenterID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(5000), got.Balance)
}

// TestTokenLedger_ConsumeAtOverdraftFloor pins the exact boundary of the
// overdraft-aware guard from the accepting side: a consumption that lands the
// balance precisely on -overdraft succeeds, and the next milli is rejected
// without touching the balance. The grace-row test probes the same floor from
// the rejecting side; together they prove the guard is `balance >= amount -
// overdraft` with no off-by-one.
func TestTokenLedger_ConsumeAtOverdraftFloor(t *testing.T) {
	db := newTokenLedgerDB(t)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	missionSvc := service.NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	notifSvc := service.NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), repository.NewPushDeviceRepositoryImpl(db))
	svc := service.NewTokenAccountServiceImpl(accountRepo, topUpRepo, centerRepo, missionSvc, notifSvc)

	// Premium tier (-30 YOTTU = 30_000 milli) via TotalPurchased >= 500_000.
	// Balance 5_000 + overdraft 30_000 = floor 35_000; consuming exactly 35_000
	// lands the balance on -30_000.
	acct := &models.TokenAccount{BeautyCenterID: 1, Balance: 5000, TotalPurchased: 500_000, Currency: "USD"}
	assert.NoError(t, accountRepo.Create(acct))

	balance, overdraft, _, err := svc.GetBalance(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(5000), balance)
	assert.Equal(t, int64(30_000), overdraft)

	// Exactly the floor: accepted, balance becomes the overdraft floor.
	assert.NoError(t, svc.Consume(1, 35_000, 0, "consume to the floor"))
	balance, _, _, err = svc.GetBalance(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(-30_000), balance, "balance may go as low as -overdraft, no further")

	// One milli past the floor: rejected, balance unchanged.
	err = svc.Consume(1, 1, 0, "one milli past the floor")
	assert.ErrorIs(t, err, apperrors.ErrInsufficientBalance)
	balance, _, _, err = svc.GetBalance(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(-30_000), balance, "rejected consume must not move the balance")
}

// TestTokenLedger_ProcessTopUpCreditsReceivedPlusBonusOnly — a pending top-up
// with legacy grace rows present must credit tokens_received + bonus_tokens
// only; no grace debt is deducted (spec: "Top-up ignores historical grace
// rows").
func TestTokenLedger_ProcessTopUpCreditsReceivedPlusBonusOnly(t *testing.T) {
	db := newTokenLedgerDB(t)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	missionSvc := service.NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	notifSvc := service.NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), repository.NewPushDeviceRepositoryImpl(db))
	svc := service.NewTokenAccountServiceImpl(accountRepo, topUpRepo, centerRepo, missionSvc, notifSvc)

	// Center + account with a starting balance.
	assert.NoError(t, db.Create(&models.BeautyCenter{ID: 1, Name: "C", Email: "c@c.com", Phone: "1", Password: "x", IsActive: true}).Error)
	assert.NoError(t, accountRepo.Create(&models.TokenAccount{BeautyCenterID: 1, Balance: 1000, Currency: "USD"}))

	// Historical grace row must be ignored by the top-up flow.
	assert.NoError(t, db.Create(&models.TokenTransaction{
		BeautyCenterID: 1, Amount: -500, Type: "grace", BalanceBefore: 1000, BalanceAfter: 1000,
	}).Error)

	topup := &models.TokenTopUp{
		BeautyCenterID: 1, AmountBs: 100, TokensReceived: 10000, BonusTokens: 2500,
		ReceiptURL: "http://test.com/r", Status: "pending",
	}
	assert.NoError(t, topUpRepo.Create(topup))

	// adminID 1 == topup.BeautyCenterID 1: the cross-center ownership check
	// (design D5) must not disturb the legacy ledger-math assertions.
	assert.NoError(t, svc.ProcessTopUp(topup.ID, 1, "approved"))

	// 1000 + 10000 + 2500 = 13500; the grace -500 was NOT re-applied.
	balance, _, _, err := svc.GetBalance(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(13500), balance)

	// TotalPurchased accumulates only the top-up part, not the pre-existing balance.
	account, err := accountRepo.FindByBeautyCenterID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(12500), account.TotalPurchased)
}

// TestTokenLedger_GetSummaryExcludesGraceRows — historical grace rows must be
// excluded from total_consumed (spec: "Summary excludes grace rows").
func TestTokenLedger_GetSummaryExcludesGraceRows(t *testing.T) {
	db := newTokenLedgerDB(t)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)

	assert.NoError(t, db.Create(&models.BeautyCenter{ID: 1, Name: "C", Email: "c@c.com", Phone: "1", Password: "x", IsActive: true}).Error)
	acct := &models.TokenAccount{BeautyCenterID: 1, Balance: 30000, Currency: "USD"}
	assert.NoError(t, accountRepo.Create(acct))

	ref := uint(1)
	_, err := accountRepo.Consume(context.Background(), acct.ID, 10000, "consumption", "service", &ref)
	assert.NoError(t, err)
	_, err = accountRepo.AddTokens(context.Background(), acct.ID, 5000, "topup", "topup", &ref)
	assert.NoError(t, err)

	// Historical grace row — must be excluded from total_consumed.
	assert.NoError(t, db.Create(&models.TokenTransaction{
		BeautyCenterID: 1, Amount: -2000, Type: "grace", BalanceBefore: 25000, BalanceAfter: 25000,
	}).Error)

	txSvc := service.NewTokenTransactionServiceImpl(repository.NewTokenTransactionRepositoryImpl(db))
	totalConsumed, totalTopUps, err := txSvc.GetSummary(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(10000), totalConsumed, "grace row must not count toward total_consumed")
	assert.Equal(t, int64(5000), totalTopUps)
}
