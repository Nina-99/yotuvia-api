package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type AccountEmailRepositoryImpl struct {
	db *gorm.DB
}

func NewAccountEmailRepositoryImpl(db *gorm.DB) AccountEmailRepository {
	return &AccountEmailRepositoryImpl{db: db}
}

func (r *AccountEmailRepositoryImpl) Reserve(ctx context.Context, email, userType string) (*models.AccountEmail, error) {
	entry := &models.AccountEmail{Email: email, UserType: userType}
	if err := r.db.WithContext(ctx).Create(entry).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailAlreadyReserved
		}
		return nil, err
	}
	return entry, nil
}

func (r *AccountEmailRepositoryImpl) BindUser(ctx context.Context, id, userID uint) error {
	return r.db.WithContext(ctx).Model(&models.AccountEmail{}).
		Where("id = ?", id).
		Update("user_id", userID).Error
}

func (r *AccountEmailRepositoryImpl) Release(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&models.AccountEmail{}, id).Error
}

func (r *AccountEmailRepositoryImpl) FindByEmail(ctx context.Context, email string) (*models.AccountEmail, error) {
	var entry models.AccountEmail
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&entry).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &entry, nil
}

// isUniqueViolation normalizes the driver-specific duplicate-key errors. GORM's
// sentinel only appears when TranslateError is enabled, so the Postgres and
// SQLite messages are matched explicitly.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicated key")
}
