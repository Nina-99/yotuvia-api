package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type OtpChallengeRepositoryImpl struct {
	db *gorm.DB
}

func NewOtpChallengeRepositoryImpl(db *gorm.DB) OtpChallengeRepository {
	return &OtpChallengeRepositoryImpl{db: db}
}

func (r *OtpChallengeRepositoryImpl) Create(ctx context.Context, challenge *models.OtpChallenge) error {
	return r.db.WithContext(ctx).Create(challenge).Error
}

func (r *OtpChallengeRepositoryImpl) FindActiveByEmail(ctx context.Context, email string, now time.Time) (*models.OtpChallenge, error) {
	var challenge models.OtpChallenge
	err := r.db.WithContext(ctx).
		Where("email = ? AND used_at IS NULL AND expires_at > ?", email, now).
		Order("created_at DESC").
		First(&challenge).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &challenge, nil
}

func (r *OtpChallengeRepositoryImpl) Consume(ctx context.Context, id uint) (bool, error) {
	res := r.db.WithContext(ctx).Model(&models.OtpChallenge{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", time.Now())
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *OtpChallengeRepositoryImpl) IncrementAttempts(ctx context.Context, id uint) (int, error) {
	if err := r.db.WithContext(ctx).Model(&models.OtpChallenge{}).
		Where("id = ?", id).
		UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error; err != nil {
		return 0, err
	}

	var challenge models.OtpChallenge
	if err := r.db.WithContext(ctx).Select("attempts").First(&challenge, id).Error; err != nil {
		return 0, err
	}
	return challenge.Attempts, nil
}

func (r *OtpChallengeRepositoryImpl) InvalidateOutstanding(ctx context.Context, email string, now time.Time) error {
	return r.db.WithContext(ctx).Model(&models.OtpChallenge{}).
		Where("email = ? AND used_at IS NULL", email).
		Update("used_at", now).Error
}

func (r *OtpChallengeRepositoryImpl) CountRecent(ctx context.Context, email string, since time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.OtpChallenge{}).
		Where("email = ? AND created_at >= ?", email, since).
		Count(&count).Error
	return count, err
}
