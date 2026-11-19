package service

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
)

// TokenHealth is the compact state the owner's dashboard shows. State is one of
// "ok" | "low" | "deficit" | "suspended".
type TokenHealth struct {
	State             string
	DeficitSince      *time.Time
	DeficitDays       int
	CanAcceptBookings bool
}

type TokenAccountService interface {
	GetOrCreate(beautyCenterID uint, currency string) (*models.TokenAccount, error)
	// GetBalance returns the balance, the reserved (held) portion and the
	// overdraft limit in milli-Ytv; the controller converts to decimal for
	// the JSON wire. Available funds are balance - reserved + overdraft.
	GetBalance(beautyCenterID uint) (balance, reserved, overdraftLimit int64, currency string, err error)
	// Reserve holds amountMilli from the account's available funds (never
	// overdraft) and caps the hold at capPct of the balance. It moves no money
	// and writes no ledger row.
	Reserve(beautyCenterID uint, amountMilli int64, capPct float64) error
	// SettleReserved spends amountMilli from an existing hold and records the
	// ledger row. It never falls back to overdraft.
	SettleReserved(beautyCenterID uint, amountMilli int64, txType string, referenceID uint, description string) error
	// ReleaseReserved returns held funds to available. It moves no money and
	// writes no ledger row.
	ReleaseReserved(beautyCenterID uint, amountMilli int64) error
	// CanConsume reports whether the balance plus the overdraft limit covers
	// amountMilli.
	CanConsume(beautyCenterID uint, amountMilli int64) (bool, error)

	// HealthSnapshot is the owner-facing view of the wallet: a single state, how
	// long the centre has been in deficit, and whether it can take bookings. It
	// exists so the dashboard does not have to re-derive the platform's policy.
	HealthSnapshot(beautyCenterID uint) (TokenHealth, error)

	// IsSuspended reports whether the platform suspended the centre for staying
	// in deficit. Suspension is separate from is_active (the owner's switch).
	IsSuspended(beautyCenterID uint) (bool, error)

	// EvaluateBalanceAlerts inspects one centre's wallet and emits at most ONE
	// notification per state episode: "running low" and "cannot take new
	// reservations". It also suspends a centre whose deficit outlived the
	// suspension window. Idempotent by construction (each alert is stamped and
	// the stamp is cleared when the state recovers).
	EvaluateBalanceAlerts(beautyCenterID uint) error

	// SweepBalanceAlerts runs EvaluateBalanceAlerts over every centre that could
	// need attention. Returns how many centres were evaluated.
	SweepBalanceAlerts(now time.Time) (int, error)
	// Consume deducts amountMilli atomically (balance + ledger row in one
	// transaction). Errors — including ErrInsufficientBalance — propagate as-is.
	Consume(beautyCenterID uint, amountMilli int64, referenceID uint, description string) error
	ProcessTopUp(topUpID, adminID uint, status string) error
	// ProcessTopUpAsPlatform applies the SAME approve/reject decision to ANY
	// centre's pending top-up. It is the platform admin's entry point: the
	// ownership check of ProcessTopUp does not apply, because a platform admin
	// oversees every centre. adminID is recorded as the validator.
	ProcessTopUpAsPlatform(topUpID, adminID uint, status string) error
	// AddTokens credits amountMilli atomically. TotalPurchased accumulates only
	// for "topup" transactions.
	AddTokens(beautyCenterID uint, amountMilli int64, txType string, referenceID uint, description string) error
	FindMyTopUps(centerID uint, page, limit int, status *string) ([]models.TokenTopUp, int64, error)
}
