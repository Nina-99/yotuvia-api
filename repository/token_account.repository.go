package repository

import (
	"context"

	"github.com/Nina-99/yottu-api/models"
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
}
