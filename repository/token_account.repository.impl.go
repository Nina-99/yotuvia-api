package repository

import (
	"context"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TokenAccountRepositoryImpl struct {
	db *gorm.DB
}

func NewTokenAccountRepositoryImpl(db *gorm.DB) TokenAccountRepository {
	return &TokenAccountRepositoryImpl{db: db}
}

func (r *TokenAccountRepositoryImpl) Create(account *models.TokenAccount) error {
	return r.db.Create(account).Error
}

func (r *TokenAccountRepositoryImpl) Update(account *models.TokenAccount) error {
	return r.db.Save(account).Error
}

func (r *TokenAccountRepositoryImpl) FindByBeautyCenterID(beautyCenterID uint) (*models.TokenAccount, error) {
	var account models.TokenAccount
	err := r.db.Where("beauty_center_id = ?", beautyCenterID).First(&account).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &account, nil
}

// Consume runs the balance deduction and the ledger insert in a single
// transaction. The account row is locked with SELECT ... FOR UPDATE so two
// concurrent consumers serialize: the loser reads the updated balance and fails
// the hard floor check with ErrInsufficientBalance. No grace row is ever
// written — failures propagate as errors and roll everything back.
func (r *TokenAccountRepositoryImpl) Consume(ctx context.Context, accountID uint, amountMilli int64,
	txnType string, description string, referenceID *uint) (*models.TokenAccount, error) {

	var account models.TokenAccount
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, accountID).Error; err != nil {
			return err
		}
		if account.Balance < amountMilli {
			return apperrors.ErrInsufficientBalance
		}

		balanceBefore := account.Balance
		account.Balance -= amountMilli
		account.TotalConsumed += amountMilli

		if err := tx.Save(&account).Error; err != nil {
			return err
		}

		ledger := &models.TokenTransaction{
			BeautyCenterID: account.BeautyCenterID,
			Amount:         -amountMilli,
			Type:           txnType,
			ReferenceID:    referenceID,
			BalanceBefore:  balanceBefore,
			BalanceAfter:   account.Balance,
			Description:    description,
		}
		return tx.Create(ledger).Error
	})
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// AddTokens credits the balance and inserts the ledger row atomically under the
// same row lock. TotalPurchased accumulates only when txnType == "topup".
func (r *TokenAccountRepositoryImpl) AddTokens(ctx context.Context, accountID uint, amountMilli int64,
	txnType string, description string, referenceID *uint) (*models.TokenAccount, error) {

	var updated *models.TokenAccount
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var err error
		updated, err = r.AddTokensInTx(tx, accountID, amountMilli, txnType, description, referenceID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// AddTokensInTx performs the credit + ledger insert inside a CALLER-managed
// transaction (feat-token-missions D7): locking read, balance update and
// ledger row commit together with whatever else the caller writes (e.g. the
// mission claim flag).
func (r *TokenAccountRepositoryImpl) AddTokensInTx(tx *gorm.DB, accountID uint, amountMilli int64,
	txnType string, description string, referenceID *uint) (*models.TokenAccount, error) {

	var account models.TokenAccount
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, accountID).Error; err != nil {
		return nil, err
	}

	balanceBefore := account.Balance
	account.Balance += amountMilli
	if txnType == "topup" {
		account.TotalPurchased += amountMilli
	}

	if err := tx.Save(&account).Error; err != nil {
		return nil, err
	}

	ledger := &models.TokenTransaction{
		BeautyCenterID: account.BeautyCenterID,
		Amount:         amountMilli,
		Type:           txnType,
		ReferenceID:    referenceID,
		BalanceBefore:  balanceBefore,
		BalanceAfter:   account.Balance,
		Description:    description,
	}
	if err := tx.Create(ledger).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// Transaction executes fn inside a GORM transaction, committing on nil
// return or rolling back on error. Used by composite operations that
// must atomically combine a status flip with a token debit.
func (r *TokenAccountRepositoryImpl) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(tx)
	})
}

// DebitInTx performs the overdraft-aware consumption inside a CALLER-managed
// transaction (feat-token-economy-config D4): locked read, guard
// `balance ≥ amount - overdraft`, decrement, TotalConsumed accumulation and
// ledger row all commit (or roll back) with the caller's completion update.
func (r *TokenAccountRepositoryImpl) DebitInTx(tx *gorm.DB, accountID uint, amountMilli int64,
	overdraftMilli int64, txnType string, description string, referenceID *uint) error {

	var account models.TokenAccount
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, accountID).Error; err != nil {
		return err
	}

	// Overdraft-aware guard: balance may go as low as -overdraftMilli.
	if account.Balance < amountMilli-overdraftMilli {
		return apperrors.ErrInsufficientBalance
	}

	balanceBefore := account.Balance
	account.Balance -= amountMilli
	account.TotalConsumed += amountMilli

	if err := tx.Save(&account).Error; err != nil {
		return err
	}

	ledger := &models.TokenTransaction{
		BeautyCenterID: account.BeautyCenterID,
		Amount:         -amountMilli,
		Type:           txnType,
		ReferenceID:    referenceID,
		BalanceBefore:  balanceBefore,
		BalanceAfter:   account.Balance,
		Description:    description,
	}
	return tx.Create(ledger).Error
}
