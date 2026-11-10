package tests

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTokenReservationDB builds a named shared-cache in-memory database with a
// single connection, so concurrent reservation tests serialize exactly like
// Postgres row locks do.
func newTokenReservationDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open reservation test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&models.TokenAccount{}, &models.TokenTransaction{}); err != nil {
		t.Fatalf("failed to migrate reservation test db: %v", err)
	}
	return db
}

func newReservationAccount(t *testing.T, db *gorm.DB, balance int64) *models.TokenAccount {
	t.Helper()
	repo := repository.NewTokenAccountRepositoryImpl(db)
	acct := &models.TokenAccount{BeautyCenterID: 1, Balance: balance, Currency: "BOB"}
	assert.NoError(t, repo.Create(acct))
	return acct
}

// TestReserve_DoesNotMoveMoney — reserving is a state change: balance and the
// ledger stay untouched (spec: "Reserving does not move money").
func TestReserve_DoesNotMoveMoney(t *testing.T) {
	db := newTokenReservationDB(t)
	repo := repository.NewTokenAccountRepositoryImpl(db)
	acct := newReservationAccount(t, db, 100000)

	updated, err := repo.Reserve(context.Background(), acct.ID, 40000, 0.50)
	assert.NoError(t, err)
	assert.Equal(t, int64(100000), updated.Balance)
	assert.Equal(t, int64(40000), updated.ReservedMilli)

	var ledgerRows int64
	db.Model(&models.TokenTransaction{}).Count(&ledgerRows)
	assert.Equal(t, int64(0), ledgerRows, "reservation must not write a ledger row")
}

// TestReserve_RejectsInsufficientAvailable — available excludes funds already
// held by another reservation.
func TestReserve_RejectsInsufficientAvailable(t *testing.T) {
	db := newTokenReservationDB(t)
	repo := repository.NewTokenAccountRepositoryImpl(db)
	acct := newReservationAccount(t, db, 10000)

	_, err := repo.Reserve(context.Background(), acct.ID, 40000, 0.50)
	assert.ErrorIs(t, err, apperrors.ErrInsufficientBalance)

	got, err := repo.FindByBeautyCenterID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), got.ReservedMilli)
}

// TestReserve_OverdraftCannotFundHold — reserve has no overdraft parameter, so
// a hold can never be financed with debt (spec: "Overdraft cannot fund a
// reservation").
func TestReserve_OverdraftCannotFundHold(t *testing.T) {
	db := newTokenReservationDB(t)
	repo := repository.NewTokenAccountRepositoryImpl(db)
	acct := newReservationAccount(t, db, 1000)

	_, err := repo.Reserve(context.Background(), acct.ID, 5000, 0.50)
	assert.ErrorIs(t, err, apperrors.ErrInsufficientBalance)
}

// TestReserve_EnforcesCap — a hold may not exceed the configured percentage of
// the balance.
func TestReserve_EnforcesCap(t *testing.T) {
	db := newTokenReservationDB(t)
	repo := repository.NewTokenAccountRepositoryImpl(db)
	acct := newReservationAccount(t, db, 100000)

	_, err := repo.Reserve(context.Background(), acct.ID, 60000, 0.50)
	assert.ErrorIs(t, err, apperrors.ErrConflict)

	got, err := repo.FindByBeautyCenterID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), got.ReservedMilli)
}

// TestReserve_ConcurrentNeverExceedsCap — two holds that together exceed the cap
// must not both succeed.
func TestReserve_ConcurrentNeverExceedsCap(t *testing.T) {
	db := newTokenReservationDB(t)
	repo := repository.NewTokenAccountRepositoryImpl(db)
	acct := newReservationAccount(t, db, 100000)

	results := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(idx int) {
			defer wg.Done()
			_, results[idx] = repo.Reserve(context.Background(), acct.ID, 30000, 0.50)
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
		}
	}
	assert.Equal(t, 1, successes, "exactly one reservation must win the cap")

	got, err := repo.FindByBeautyCenterID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(30000), got.ReservedMilli, "cap must never be exceeded")
}

// TestSettleReserved_HappyPath — settling decrements balance and reservation and
// writes one boost_spend ledger row atomically.
func TestSettleReserved_HappyPath(t *testing.T) {
	db := newTokenReservationDB(t)
	repo := repository.NewTokenAccountRepositoryImpl(db)
	acct := newReservationAccount(t, db, 100000)

	_, err := repo.Reserve(context.Background(), acct.ID, 40000, 0.50)
	assert.NoError(t, err)

	ref := uint(7)
	updated, err := repo.SettleReserved(context.Background(), acct.ID, 500, "boost_spend", "impression", &ref)
	assert.NoError(t, err)
	assert.Equal(t, int64(99500), updated.Balance)
	assert.Equal(t, int64(39500), updated.ReservedMilli)
	assert.Equal(t, int64(500), updated.TotalConsumed)

	var txs []models.TokenTransaction
	assert.NoError(t, db.Find(&txs).Error)
	assert.Len(t, txs, 1)
	assert.Equal(t, int64(-500), txs[0].Amount)
	assert.Equal(t, "boost_spend", txs[0].Type)
	assert.Equal(t, int64(100000), txs[0].BalanceBefore)
	assert.Equal(t, int64(99500), txs[0].BalanceAfter)
	assert.Equal(t, "7", fmt.Sprint(*txs[0].ReferenceID))
}

// TestSettleReserved_RejectsBeyondHold — a settlement larger than the hold is
// rejected and leaves both balance and ledger untouched.
func TestSettleReserved_RejectsBeyondHold(t *testing.T) {
	db := newTokenReservationDB(t)
	repo := repository.NewTokenAccountRepositoryImpl(db)
	acct := newReservationAccount(t, db, 100000)

	_, err := repo.Reserve(context.Background(), acct.ID, 1000, 0.50)
	assert.NoError(t, err)

	_, err = repo.SettleReserved(context.Background(), acct.ID, 2000, "boost_spend", "overspend", nil)
	assert.ErrorIs(t, err, apperrors.ErrInsufficientBalance)

	got, err := repo.FindByBeautyCenterID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(100000), got.Balance)
	assert.Equal(t, int64(1000), got.ReservedMilli)

	var ledgerRows int64
	db.Model(&models.TokenTransaction{}).Count(&ledgerRows)
	assert.Equal(t, int64(0), ledgerRows)
}

// TestSettleReserved_ConcurrentNeverOverspendsTheHold — two settlements that
// cannot both fit in the hold must serialize.
func TestSettleReserved_ConcurrentNeverOverspendsTheHold(t *testing.T) {
	db := newTokenReservationDB(t)
	repo := repository.NewTokenAccountRepositoryImpl(db)
	acct := newReservationAccount(t, db, 100000)

	_, err := repo.Reserve(context.Background(), acct.ID, 40000, 0.50)
	assert.NoError(t, err)

	results := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(idx int) {
			defer wg.Done()
			_, results[idx] = repo.SettleReserved(context.Background(), acct.ID, 25000, "boost_spend", "concurrent", nil)
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, apperrors.ErrInsufficientBalance) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	assert.Equal(t, 1, successes)

	got, err := repo.FindByBeautyCenterID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(75000), got.Balance, "only one settlement may debit")
	assert.Equal(t, int64(15000), got.ReservedMilli)
}

// TestReleaseReserved_ReturnsToAvailableWithoutLedger — release only lowers the
// hold; balance is untouched and no ledger row is written.
func TestReleaseReserved_ReturnsToAvailableWithoutLedger(t *testing.T) {
	db := newTokenReservationDB(t)
	repo := repository.NewTokenAccountRepositoryImpl(db)
	acct := newReservationAccount(t, db, 100000)

	_, err := repo.Reserve(context.Background(), acct.ID, 40000, 0.50)
	assert.NoError(t, err)

	updated, err := repo.ReleaseReserved(context.Background(), acct.ID, 25000)
	assert.NoError(t, err)
	assert.Equal(t, int64(100000), updated.Balance)
	assert.Equal(t, int64(15000), updated.ReservedMilli)

	// Releasing zero is a no-op (campaign already settled/released).
	updated, err = repo.ReleaseReserved(context.Background(), acct.ID, 0)
	assert.NoError(t, err)
	assert.Equal(t, int64(15000), updated.ReservedMilli)

	// Releasing more than held is rejected, never silently clamped.
	_, err = repo.ReleaseReserved(context.Background(), acct.ID, 20000)
	assert.ErrorIs(t, err, apperrors.ErrConflict)

	var ledgerRows int64
	db.Model(&models.TokenTransaction{}).Count(&ledgerRows)
	assert.Equal(t, int64(0), ledgerRows)
}

// TestReservation_KeepsBalanceEqualToLedgerSum — the core invariant: holds never
// desync the ledger from the balance. The account starts with a seeded balance,
// so the invariant is checked as balance delta = ledger sum.
func TestReservation_KeepsBalanceEqualToLedgerSum(t *testing.T) {
	db := newTokenReservationDB(t)
	repo := repository.NewTokenAccountRepositoryImpl(db)
	acct := newReservationAccount(t, db, 100000)

	_, err := repo.Reserve(context.Background(), acct.ID, 40000, 0.50)
	assert.NoError(t, err)
	_, err = repo.SettleReserved(context.Background(), acct.ID, 1500, "boost_spend", "impression", nil)
	assert.NoError(t, err)

	var sum int64
	assert.NoError(t, db.Model(&models.TokenTransaction{}).Select("COALESCE(SUM(amount),0)").Scan(&sum).Error)
	got, err := repo.FindByBeautyCenterID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(100000)+sum, got.Balance, "balance delta must equal the ledger sum")
}
