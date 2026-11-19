package repository

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
)

// TopUpReconciliation is the platform's read model for the manual money path:
// how much is waiting, how old it is, and whether any approved top-up was
// credited a different amount than it declares (ledger drift). Amounts are in
// the stored units (amount_bs is fiat, tokens are milli-Ytv).
type TopUpReconciliation struct {
	PendingCount     int64
	PendingAmountBs  float64
	PendingOver24h   int64   // pending rows older than 24h (aging / SLA breach)
	OldestPendingAge float64 // hours; 0 when nothing is pending
	ApprovedCount    int64
	ApprovedAmountBs float64
	ApprovedTokens   int64 // sum of tokens_received + bonus_tokens (milli)
	RejectedCount    int64
	RejectedAmountBs float64
	DriftCount       int64 // approved top-ups whose ledger credit != declared tokens
}

type TokenTopUpRepository interface {
	Create(topup *models.TokenTopUp) error
	Update(topup *models.TokenTopUp) error
	FindByID(id uint) (*models.TokenTopUp, error)
	FindByBeautyCenterID(beautyCenterID uint) ([]models.TokenTopUp, error)
	FindByCenterIDWithStatus(beautyCenterID uint, page, limit int, status string) ([]models.TokenTopUp, int64, error)
	FindPending() ([]models.TokenTopUp, error)
	FindAll(page, limit int) ([]models.TokenTopUp, int64, error)
	// FindAllWithFilters is the platform-wide queue read: it lists top-ups across
	// EVERY centre, with an optional centre filter (nil = all) and an optional
	// status filter ("" = all). Both filters are applied in the SQL query, so the
	// caller never has to re-filter in memory and `total` always matches.
	FindAllWithFilters(page, limit int, beautyCenterID *uint, status string) ([]models.TokenTopUp, int64, error)

	// Reconciliation aggregates the manual money path for the platform panel:
	// per-status totals, pending aging, and ledger drift (approved top-ups whose
	// credited tokens differ from what they declare). now anchors the aging math.
	Reconciliation(now time.Time) (*TopUpReconciliation, error)
}
