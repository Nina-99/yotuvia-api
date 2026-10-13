package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTokenLedgerDB builds a minimal in-memory SQLite DB for the token ledger
// integration tests. The atomicity/concurrency/grace suites each get their own
// database so GORM callback injections never leak across tests.
func newTokenLedgerDB(t *testing.T) *gorm.DB {
	t.Helper()
	// Service paths log through utils.Log; the tests package has no TestMain,
	// so install a nop logger like the QR suite does.
	utils.Log = zap.NewNop().Sugar()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open token ledger test db: %v", err)
	}
	if err := db.AutoMigrate(&models.TokenAccount{}, &models.TokenTransaction{}, &models.TokenTopUp{}, &models.BeautyCenter{}); err != nil {
		t.Fatalf("failed to migrate token ledger test db: %v", err)
	}
	return db
}

// TestTokenLedger_AtomicConsumeRollsBackOnLedgerInsertFailure — a ledger insert
// failure inside the transaction must roll back the balance change: no balance
// change without a ledger row (spec: "Failed ledger insert rolls back the
// balance change").
func TestTokenLedger_AtomicConsumeRollsBackOnLedgerInsertFailure(t *testing.T) {
	db := newTokenLedgerDB(t)
	repo := repository.NewTokenAccountRepositoryImpl(db)

	acct := &models.TokenAccount{BeautyCenterID: 1, Balance: 15000, Currency: "USD"}
	assert.NoError(t, repo.Create(acct))

	// Inject a failure on the ledger row insert only; the balance UPDATE (Save)
	// inside the same transaction must still be rolled back.
	err := db.Callback().Create().Before("gorm:create").Register("fail_ledger_insert", func(d *gorm.DB) {
		if _, ok := d.Statement.Model.(*models.TokenTransaction); ok {
			d.AddError(errors.New("injected ledger insert failure"))
		}
	})
	assert.NoError(t, err)

	ref := uint(1)
	_, consumeErr := repo.Consume(context.Background(), acct.ID, 10000, "consumption", "atomic test", &ref)
	assert.Error(t, consumeErr)
	assert.ErrorContains(t, consumeErr, "injected ledger insert failure")

	// Balance unchanged and no ledger row: the transaction rolled back both.
	got, err := repo.FindByBeautyCenterID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(15000), got.Balance)

	var ledgerRows int64
	db.Model(&models.TokenTransaction{}).Where("beauty_center_id = ?", 1).Count(&ledgerRows)
	assert.Equal(t, int64(0), ledgerRows)
}

// TestTokenLedger_AtomicConsumeHappyPath — with no injected failure the same
// call commits balance + ledger row atomically and records the correct
// balance_before / balance_after (spec: "Successful consumption commits
// atomically").
func TestTokenLedger_AtomicConsumeHappyPath(t *testing.T) {
	db := newTokenLedgerDB(t)
	repo := repository.NewTokenAccountRepositoryImpl(db)

	acct := &models.TokenAccount{BeautyCenterID: 1, Balance: 15000, Currency: "USD"}
	assert.NoError(t, repo.Create(acct))

	ref := uint(1)
	updated, err := repo.Consume(context.Background(), acct.ID, 10000, "consumption", "atomic test", &ref)
	assert.NoError(t, err)
	assert.Equal(t, int64(5000), updated.Balance)

	// Balance persisted, ledger row committed with matching before/after.
	got, err := repo.FindByBeautyCenterID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(5000), got.Balance)

	var txs []models.TokenTransaction
	assert.NoError(t, db.Where("beauty_center_id = ?", 1).Find(&txs).Error)
	assert.Len(t, txs, 1)
	assert.Equal(t, int64(-10000), txs[0].Amount)
	assert.Equal(t, "consumption", txs[0].Type)
	assert.Equal(t, int64(15000), txs[0].BalanceBefore)
	assert.Equal(t, int64(5000), txs[0].BalanceAfter)
}
