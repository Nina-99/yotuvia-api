package repository

import (
	"context"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/models"
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

	var account models.TokenAccount
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, accountID).Error; err != nil {
			return err
		}

		balanceBefore := account.Balance
		account.Balance += amountMilli
		if txnType == "topup" {
			account.TotalPurchased += amountMilli
		}

		if err := tx.Save(&account).Error; err != nil {
			return err
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
		return tx.Create(ledger).Error
	})
	if err != nil {
		return nil, err
	}
	return &account, nil
}
