package repository

import (
	"context"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type TokenAccountRepository interface {
	Create(account *models.TokenAccount) error
	Update(account *models.TokenAccount) error
	FindByBeautyCenterID(beautyCenterID uint) (*models.TokenAccount, error)

	// Consume deducts amountMilli from the account balance and inserts the
	// ledger row atomically: one transaction locks the account row (FOR UPDATE)
	// so concurrent consumers cannot double-spend, and any failure rolls back
	// both the balance change and the ledger insert.
	Consume(ctx context.Context, accountID uint, amountMilli int64,
		txnType string, description string, referenceID *uint) (*models.TokenAccount, error)

	// AddTokens credits amountMilli to the account balance and inserts the
	// ledger row atomically. TotalPurchased accumulates only for "topup"
	// transactions — bonuses, welcome, mission and referral credits are not
	// purchases.
	AddTokens(ctx context.Context, accountID uint, amountMilli int64,
		txnType string, description string, referenceID *uint) (*models.TokenAccount, error)

	// AddTokensInTx is the transaction-aware variant: it performs the same
	// credit + ledger insert inside a CALLER-managed transaction so composite
	// operations (e.g. mission claim: flag + credit) commit as one unit.
	AddTokensInTx(tx *gorm.DB, accountID uint, amountMilli int64,
		txnType string, description string, referenceID *uint) (*models.TokenAccount, error)

	// DebitInTx is the overdraft-aware transactional counterpart used by QR
	// completion (feat-token-economy-config): locks the account row and
	// decrements balance while `balance ≥ amount - overdraft`, incrementing
	// TotalConsumed and inserting the consumption ledger row. Fails with
	// apperrors.ErrInsufficientBalance when the guard rejects — the CALLER's
	// transaction rolls back with it.
	DebitInTx(tx *gorm.DB, accountID uint, amountMilli int64, overdraftMilli int64,
		txnType string, description string, referenceID *uint) error

	// ReserveInTx holds amountMilli from the account's available funds
	// (balance - reserved). It never moves money and never writes a ledger row.
	// It fails with ErrInsufficientBalance when available funds do not cover the
	// amount (overdraft is NOT usable for holds) and with ErrConflict when the
	// resulting hold would exceed capPct of the balance (capPct <= 0 disables the
	// cap). The guard is a single conditional UPDATE, so concurrent reserves
	// cannot overshoot.
	ReserveInTx(tx *gorm.DB, accountID uint, amountMilli int64, capPct float64) error

	// Reserve is the transaction-owning variant of ReserveInTx. It returns the
	// updated account.
	Reserve(ctx context.Context, accountID uint, amountMilli int64, capPct float64) (*models.TokenAccount, error)

	// SettleReservedInTx spends amountMilli from an existing hold: it decrements
	// balance and reserved, accumulates TotalConsumed and inserts the ledger row
	// in the CALLER's transaction. It fails with ErrInsufficientBalance when the
	// hold no longer covers the amount, and never falls back to overdraft.
	SettleReservedInTx(tx *gorm.DB, accountID uint, amountMilli int64,
		txnType string, description string, referenceID *uint) error

	// SettleReserved is the transaction-owning variant of SettleReservedInTx.
	SettleReserved(ctx context.Context, accountID uint, amountMilli int64,
		txnType string, description string, referenceID *uint) (*models.TokenAccount, error)

	// ReleaseReservedInTx returns held funds to available without moving money or
	// writing a ledger row. Releasing zero is a no-op; releasing more than the
	// current hold fails with ErrConflict instead of silently clamping.
	ReleaseReservedInTx(tx *gorm.DB, accountID uint, amountMilli int64) error

	// ReleaseReserved is the transaction-owning variant of ReleaseReservedInTx.
	ReleaseReserved(ctx context.Context, accountID uint, amountMilli int64) (*models.TokenAccount, error)

	// Transaction executes fn inside a GORM transaction, committing on
	// nil return or rolling back on error. Used by composite operations
	// that must atomically combine a status flip with a token debit.
	Transaction(fn func(tx *gorm.DB) error) error
}
