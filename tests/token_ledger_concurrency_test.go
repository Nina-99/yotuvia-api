package tests

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTokenLedgerConcurrencyDB uses a named shared-cache in-memory database with
// a single connection so the two goroutines serialize on the same ledger state
// exactly like Postgres row locks serialize on FOR UPDATE.
func newTokenLedgerConcurrencyDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open token ledger concurrency db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&models.TokenAccount{}, &models.TokenTransaction{}); err != nil {
		t.Fatalf("failed to migrate token ledger concurrency db: %v", err)
	}
	return db
}

// TestTokenLedger_ConcurrentConsumesCannotDoubleSpend — two concurrent
// Consume(10000) calls on a 15000 balance must serialize: final balance 5000
// with exactly one successful deduction and one insufficient-balance error
// (spec: "Concurrent consumes cannot double-spend").
func TestTokenLedger_ConcurrentConsumesCannotDoubleSpend(t *testing.T) {
	db := newTokenLedgerConcurrencyDB(t)
	repo := repository.NewTokenAccountRepositoryImpl(db)

	acct := &models.TokenAccount{BeautyCenterID: 1, Balance: 15000, Currency: "USD"}
	assert.NoError(t, repo.Create(acct))

	results := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(idx int) {
			defer wg.Done()
			ref := uint(10 + idx)
			_, results[idx] = repo.Consume(context.Background(), acct.ID, 10000, "consumption", "concurrent", &ref)
		}(i)
	}
	wg.Wait()

	// Exactly one success and one insufficient-balance error.
	successes, insuff := 0, 0
	for _, err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, apperrors.ErrInsufficientBalance) {
			insuff++
		}
	}
	assert.Equal(t, 1, successes, "exactly one deduction must succeed")
	assert.Equal(t, 1, insuff, "the losing request must fail with insufficient balance")

	// Final balance is 5000 and exactly one ledger row was committed.
	got, err := repo.FindByBeautyCenterID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(5000), got.Balance)

	var ledgerRows int64
	db.Model(&models.TokenTransaction{}).Where("beauty_center_id = ?", 1).Count(&ledgerRows)
	assert.Equal(t, int64(1), ledgerRows)
}
