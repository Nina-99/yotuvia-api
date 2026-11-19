package tests

// The overdraft is DEFERRED payment, not free money: the balance is a single
// signed number, so the next top-up credits it as-is and the debt is absorbed.
// Without that property the whole credit line would be an exploit — a centre
// could overdraw its -30 Ytv and walk away.
//
// This pins the invariant, because it is the only thing standing between
// "overdraft" and "free tokens".

import (
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newOverdraftRepaymentSvc(t *testing.T) (service.TokenAccountService, repository.TokenAccountRepository) {
	t.Helper()
	db := newTokenLedgerDB(t)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	missionSvc := service.NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	notifSvc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)
	return service.NewTokenAccountServiceImpl(accountRepo, topUpRepo, centerRepo, missionSvc, notifSvc), accountRepo
}

// TestOverdraft_IsDeductedFromTheNextTopUp — the core economic invariant.
func TestOverdraft_IsDeductedFromTheNextTopUp(t *testing.T) {
	svc, accountRepo := newOverdraftRepaymentSvc(t)

	// Premium tier (-30 Ytv = 30_000 milli) so the floor is deterministic.
	acct := &models.TokenAccount{
		BeautyCenterID: 1, Balance: 0, TotalPurchased: 500_000, Currency: "USD",
	}
	require.NoError(t, accountRepo.Create(acct))

	// Spend 20 Ytv of commission the centre does not have: the balance goes to
	// -20 Ytv thanks to the overdraft.
	require.NoError(t, svc.Consume(1, utils.ToMilli(20), 0, "commission"))
	balance, _, overdraft, _, err := svc.GetBalance(1)
	require.NoError(t, err)
	assert.Equal(t, utils.ToMilli(-20), balance, "balance must be allowed to go negative")
	assert.Equal(t, utils.ToMilli(30), overdraft)

	// Buy 50 Ytv. The debt is absorbed by the arithmetic, with no separate
	// "pay the debt first" step.
	require.NoError(t, svc.AddTokens(1, utils.ToMilli(50), "topup", 1, "top-up approved"))

	balance, _, _, _, err = svc.GetBalance(1)
	require.NoError(t, err)
	assert.Equal(t, utils.ToMilli(30), balance,
		"50 bought minus 20 of debt must leave 30, not 50")

	// And the centre can keep spending down to the floor again.
	require.NoError(t, svc.Consume(1, utils.ToMilli(60), 0, "commission"))
	balance, _, _, _, err = svc.GetBalance(1)
	require.NoError(t, err)
	assert.Equal(t, utils.ToMilli(-30), balance, "exactly at the overdraft floor")
}

// TestOverdraft_CannotBeResetByAbandoningTheAccount — the balance is the debt.
// There is no separate debt field to forgive: recreating or re-reading the
// account keeps the negative balance until a top-up covers it.
func TestOverdraft_SurvivesReload(t *testing.T) {
	svc, accountRepo := newOverdraftRepaymentSvc(t)

	require.NoError(t, accountRepo.Create(&models.TokenAccount{
		BeautyCenterID: 1, Balance: 0, TotalPurchased: 500_000, Currency: "USD",
	}))
	require.NoError(t, svc.Consume(1, utils.ToMilli(12), 0, "commission"))

	// A fresh read (new request, new session) sees the debt.
	balance, _, _, _, err := svc.GetBalance(1)
	require.NoError(t, err)
	assert.Equal(t, utils.ToMilli(-12), balance)
}
