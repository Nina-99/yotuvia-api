package service

import "github.com/Nina-99/yotuvia-api/models"

type TokenAccountService interface {
	GetOrCreate(beautyCenterID uint, currency string) (*models.TokenAccount, error)
	// GetBalance returns the balance and overdraft limit in milli-YOTTUs; the
	// controller converts to decimal for the JSON wire.
	GetBalance(beautyCenterID uint) (balance, overdraftLimit int64, currency string, err error)
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
