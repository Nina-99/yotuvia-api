package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type AuthTokenRepositoryImpl struct {
	db *gorm.DB
}

func NewAuthTokenRepositoryImpl(db *gorm.DB) AuthTokenRepository {
	return &AuthTokenRepositoryImpl{db: db}
}

func (r *AuthTokenRepositoryImpl) Create(ctx context.Context, token *models.AuthToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *AuthTokenRepositoryImpl) FindByHash(ctx context.Context, hash string) (*models.AuthToken, error) {
	var token models.AuthToken
	err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &token, nil
}

func (r *AuthTokenRepositoryImpl) Consume(ctx context.Context, id uint) (bool, error) {
	res := r.db.WithContext(ctx).Model(&models.AuthToken{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", time.Now())
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *AuthTokenRepositoryImpl) InvalidateOutstanding(ctx context.Context, userType string, userID uint, purpose string) error {
	return r.db.WithContext(ctx).Model(&models.AuthToken{}).
		Where("user_type = ? AND user_id = ? AND purpose = ? AND used_at IS NULL", userType, userID, purpose).
		Update("used_at", time.Now()).Error
}
