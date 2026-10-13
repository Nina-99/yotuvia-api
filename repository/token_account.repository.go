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

	// Transaction executes fn inside a GORM transaction, committing on
	// nil return or rolling back on error. Used by composite operations
	// that must atomically combine a status flip with a token debit.
	Transaction(fn func(tx *gorm.DB) error) error
}
