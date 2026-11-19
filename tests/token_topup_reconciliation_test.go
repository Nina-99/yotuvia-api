package tests

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/stretchr/testify/assert"
)

// TestTopUpReconciliation_AggregatesBacklogAgingAndDrift seeds a mixed queue and
// asserts the platform read model: per-status totals, the pending backlog with
// its 24h breach count and oldest age, and ledger drift (approved top-ups whose
// credited tokens differ from the declared amount).
func TestTopUpReconciliation_AggregatesBacklogAgingAndDrift(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewTokenTopUpRepositoryImpl(db)

	now := time.Now()

	// Two pending: one fresh, one 48h old (SLA breach).
	fresh := seedPendingTopUp(t, db, 1)
	old := seedPendingTopUp(t, db, 1)
	assert.NoError(t, db.Model(&models.TokenTopUp{}).Where("id = ?", old.ID).
		Update("created_at", now.Add(-48*time.Hour)).Error)

	// Approved with a CORRECT ledger credit (12500 == 10000 + 2500).
	okTopUp := seedPendingTopUp(t, db, 1)
	assert.NoError(t, db.Model(&models.TokenTopUp{}).Where("id = ?", okTopUp.ID).
		Updates(map[string]interface{}{"status": "approved"}).Error)
	assert.NoError(t, db.Create(&models.TokenTransaction{
		BeautyCenterID: 1, Amount: 12500, Type: "topup", ReferenceID: &okTopUp.ID,
	}).Error)

	// Approved with NO ledger credit -> drift.
	drift := seedPendingTopUp(t, db, 2)
	assert.NoError(t, db.Model(&models.TokenTopUp{}).Where("id = ?", drift.ID).
		Updates(map[string]interface{}{"status": "approved"}).Error)

	// Rejected.
	rejected := seedPendingTopUp(t, db, 2)
	assert.NoError(t, db.Model(&models.TokenTopUp{}).Where("id = ?", rejected.ID).
		Updates(map[string]interface{}{"status": "rejected"}).Error)

	_ = fresh

	rec, err := repo.Reconciliation(now)
	assert.NoError(t, err)

	assert.Equal(t, int64(2), rec.PendingCount, "two pending top-ups")
	assert.InDelta(t, 200.0, rec.PendingAmountBs, 0.001, "100 + 100 fiat")
	assert.Equal(t, int64(1), rec.PendingOver24h, "the 48h-old one breaches 24h")
	assert.Greater(t, rec.OldestPendingAge, 47.0, "oldest pending is ~48h old")

	assert.Equal(t, int64(2), rec.ApprovedCount)
	assert.InDelta(t, 200.0, rec.ApprovedAmountBs, 0.001)
	assert.Equal(t, int64(25000), rec.ApprovedTokens, "2 x (10000 + 2500) milli")

	assert.Equal(t, int64(1), rec.RejectedCount)
	assert.InDelta(t, 100.0, rec.RejectedAmountBs, 0.001)

	assert.Equal(t, int64(1), rec.DriftCount, "the approved top-up with no ledger is drift")
}

// TestTopUpReconciliation_EmptyIsZero guards the empty-platform case: no rows
// must yield a zeroed struct, not a NULL scan error.
func TestTopUpReconciliation_EmptyIsZero(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewTokenTopUpRepositoryImpl(db)

	rec, err := repo.Reconciliation(time.Now())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), rec.PendingCount)
	assert.Equal(t, int64(0), rec.ApprovedCount)
	assert.Equal(t, int64(0), rec.DriftCount)
	assert.Zero(t, rec.OldestPendingAge)
}
