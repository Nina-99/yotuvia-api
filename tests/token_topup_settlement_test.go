package tests

import (
	"sync"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// newTopupSettlementService wires the real repos over the shared in-memory DB so
// the settlement path exercises the actual transaction, not a stub.
func newTopupSettlementService(db *gorm.DB) service.TokenAccountService {
	utils.Log = zap.NewNop().Sugar()
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	missionSvc := service.NewTokenMissionServiceImpl(
		repository.NewTokenMissionRepositoryImpl(db), nil, accountRepo,
	)
	return service.NewTokenAccountServiceImpl(
		accountRepo,
		repository.NewTokenTopUpRepositoryImpl(db),
		repository.NewBeautyCenterRepositoryImpl(db),
		missionSvc,
		service.NewNotificationServiceImpl(
			repository.NewNotificationRepositoryImpl(db),
			repository.NewPushDeviceRepositoryImpl(db),
		),
	)
}

// TestTopUpApproval_ConcurrentApprovalsCreditOnce is the money-safety core: two
// operators approving the SAME pending top-up at the same time must credit the
// balance exactly once. The check-then-act flow let both pass the "pending"
// guard and double-credit; the atomic conditional transition must serialize them.
func TestTopUpApproval_ConcurrentApprovalsCreditOnce(t *testing.T) {
	db := setupRouterAuthzDB(t)
	assert.NoError(t, db.Create(&models.TokenAccount{
		BeautyCenterID: 1, Balance: 0, Currency: "BOB",
	}).Error)
	topup := seedPendingTopUp(t, db, 1) // TokensReceived 10000 + Bonus 2500 = 12500
	svc := newTopupSettlementService(db)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = svc.ProcessTopUpAsPlatform(topup.ID, 99, "approved")
		}(i)
	}
	wg.Wait()

	success := 0
	for _, e := range errs {
		if e == nil {
			success++
		}
	}
	assert.Equal(t, 1, success, "exactly one approval may win; the other must be rejected")

	var acct models.TokenAccount
	assert.NoError(t, db.Where("beauty_center_id = ?", 1).First(&acct).Error)
	assert.Equal(t, int64(12500), acct.Balance, "the balance must be credited exactly once")

	var ledgerRows int64
	db.Model(&models.TokenTransaction{}).
		Where("type = ? AND reference_id = ?", "topup", topup.ID).
		Count(&ledgerRows)
	assert.Equal(t, int64(1), ledgerRows, "exactly one topup ledger row")

	var stored models.TokenTopUp
	assert.NoError(t, db.First(&stored, topup.ID).Error)
	assert.Equal(t, "approved", stored.Status)
}

// TestTopUpApproval_MissingAccountRollsBack proves the status flip and the
// credit are ONE transaction: if the credit cannot happen, the top-up must not
// be left "approved" (the old flow marked it approved first and lost the money).
func TestTopUpApproval_MissingAccountRollsBack(t *testing.T) {
	db := setupRouterAuthzDB(t)
	// Deliberately no TokenAccount for center 42.
	topup := seedPendingTopUp(t, db, 42)
	svc := newTopupSettlementService(db)

	err := svc.ProcessTopUpAsPlatform(topup.ID, 99, "approved")
	assert.Error(t, err, "approval must fail when the account cannot be credited")

	var stored models.TokenTopUp
	assert.NoError(t, db.First(&stored, topup.ID).Error)
	assert.Equal(t, "pending", stored.Status, "a failed credit must not leave the top-up approved")

	var ledgerRows int64
	db.Model(&models.TokenTransaction{}).
		Where("type = ? AND reference_id = ?", "topup", topup.ID).
		Count(&ledgerRows)
	assert.Equal(t, int64(0), ledgerRows, "no ledger row on a rolled-back approval")
}
