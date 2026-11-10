package service

import "github.com/Nina-99/yotuvia-api/models"

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
