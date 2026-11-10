package repository

import (
	"context"
	"errors"
	"math"
	"time"

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

	// Overdraft-aware guard: balance may go as low as -overdraftMilli, but the
	// funds held by a reservation are not spendable — commission can never
	// consume a boost hold.
	if account.Balance-account.ReservedMilli < amountMilli-overdraftMilli {
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

// findByID loads one account by primary key. A missing row returns (nil, nil) so
// callers can distinguish "not found" from a database error.
func (r *TokenAccountRepositoryImpl) findByID(accountID uint) (*models.TokenAccount, error) {
	var account models.TokenAccount
	if err := r.db.First(&account, accountID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &account, nil
}

// ReserveInTx holds amountMilli from available funds with ONE conditional
// UPDATE: `balance - reserved >= amount` plus, when capPct > 0,
// `(reserved + amount) * 100 <= balance * capBps`. The conditional form is what
// makes concurrent reserves safe — two goroutines can never both pass a stale
// read of reserved_milli.
func (r *TokenAccountRepositoryImpl) ReserveInTx(tx *gorm.DB, accountID uint, amountMilli int64, capPct float64) error {
	if amountMilli <= 0 {
		return nil
	}

	query := `UPDATE token_accounts
		SET reserved_milli = reserved_milli + ?, updated_at = ?
		WHERE id = ? AND balance - reserved_milli >= ?`
	args := []interface{}{amountMilli, time.Now(), accountID, amountMilli}

	if capPct > 0 {
		// Integer form of the percentage cap: avoid float comparison in SQL.
		capBps := int64(math.Round(capPct * 100))
		query += ` AND (reserved_milli + ?) * 100 <= balance * ?`
		args = append(args, amountMilli, capBps)
	}

	res := tx.Exec(query, args...)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 1 {
		return nil
	}

	// No row updated: classify without mutating anything. A fresh read inside
	// the caller's transaction is enough because the UPDATE already proved the
	// guard did not pass.
	var account models.TokenAccount
	if err := tx.First(&account, accountID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("token account not found")
		}
		return err
	}
	if account.Balance-account.ReservedMilli < amountMilli {
		return apperrors.ErrInsufficientBalance
	}
	return apperrors.ErrConflict
}

// SettleReservedInTx spends from an existing hold. The conditional UPDATE
// decrements balance and reserved together, so a settlement can never take
// funds that were not held and can never fall back to overdraft.
func (r *TokenAccountRepositoryImpl) SettleReservedInTx(tx *gorm.DB, accountID uint, amountMilli int64,
	txnType string, description string, referenceID *uint) error {

	if amountMilli <= 0 {
		return nil
	}

	res := tx.Exec(`UPDATE token_accounts
		SET balance = balance - ?, reserved_milli = reserved_milli - ?,
		    total_consumed = total_consumed + ?, updated_at = ?
		WHERE id = ? AND reserved_milli >= ?`,
		amountMilli, amountMilli, amountMilli, time.Now(), accountID, amountMilli)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var count int64
		if err := tx.Model(&models.TokenAccount{}).Where("id = ?", accountID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errors.New("token account not found")
		}
		return apperrors.ErrInsufficientBalance
	}

	// The UPDATE holds the row lock until commit, so this read returns exactly
	// the values this settlement wrote.
	var account models.TokenAccount
	if err := tx.First(&account, accountID).Error; err != nil {
		return err
	}
	balanceAfter := account.Balance
	ledger := &models.TokenTransaction{
		BeautyCenterID: account.BeautyCenterID,
		Amount:         -amountMilli,
		Type:           txnType,
		ReferenceID:    referenceID,
		BalanceBefore:  balanceAfter + amountMilli,
		BalanceAfter:   balanceAfter,
		Description:    description,
	}
	return tx.Create(ledger).Error
}

// ReleaseReservedInTx returns held funds to available. No money moves and no
// ledger row is written. Releasing zero is a no-op; an over-release is refused
// rather than clamped, so a caller bug never silently corrupts the hold.
func (r *TokenAccountRepositoryImpl) ReleaseReservedInTx(tx *gorm.DB, accountID uint, amountMilli int64) error {
	if amountMilli <= 0 {
		return nil
	}

	res := tx.Exec(`UPDATE token_accounts
		SET reserved_milli = reserved_milli - ?, updated_at = ?
		WHERE id = ? AND reserved_milli >= ?`,
		amountMilli, time.Now(), accountID, amountMilli)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 1 {
		return nil
	}

	var account models.TokenAccount
	if err := tx.First(&account, accountID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("token account not found")
		}
		return err
	}
	// Nothing held → the campaign already released. Idempotent no-op.
	if account.ReservedMilli == 0 {
		return nil
	}
	return apperrors.ErrConflict
}

// Reserve is the transaction-owning variant of ReserveInTx.
func (r *TokenAccountRepositoryImpl) Reserve(ctx context.Context, accountID uint, amountMilli int64, capPct float64) (*models.TokenAccount, error) {
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		return r.ReserveInTx(tx, accountID, amountMilli, capPct)
	}); err != nil {
		return nil, err
	}
	return r.findByID(accountID)
}

// SettleReserved is the transaction-owning variant of SettleReservedInTx.
func (r *TokenAccountRepositoryImpl) SettleReserved(ctx context.Context, accountID uint, amountMilli int64,
	txnType string, description string, referenceID *uint) (*models.TokenAccount, error) {

	if err := r.db.Transaction(func(tx *gorm.DB) error {
		return r.SettleReservedInTx(tx, accountID, amountMilli, txnType, description, referenceID)
	}); err != nil {
		return nil, err
	}
	return r.findByID(accountID)
}

// ReleaseReserved is the transaction-owning variant of ReleaseReservedInTx.
func (r *TokenAccountRepositoryImpl) ReleaseReserved(ctx context.Context, accountID uint, amountMilli int64) (*models.TokenAccount, error) {
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		return r.ReleaseReservedInTx(tx, accountID, amountMilli)
	}); err != nil {
		return nil, err
	}
	return r.findByID(accountID)
}
