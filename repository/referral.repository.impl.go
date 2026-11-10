package repository

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type ReferralRepositoryImpl struct {
	db *gorm.DB
}

func NewReferralRepositoryImpl(db *gorm.DB) ReferralRepository {
	return &ReferralRepositoryImpl{db: db}
}

func (r *ReferralRepositoryImpl) Create(referral *models.Referral) error {
	return r.db.Create(referral).Error
}

func (r *ReferralRepositoryImpl) Update(referral *models.Referral) error {
	return r.db.Save(referral).Error
}

func (r *ReferralRepositoryImpl) FindByID(id uint) (*models.Referral, error) {
	var referral models.Referral
	err := r.db.First(&referral, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &referral, nil
}

func (r *ReferralRepositoryImpl) FindByReferredID(referredID uint) (*models.Referral, error) {
	var referral models.Referral
	err := r.db.Where("referred_id = ?", referredID).First(&referral).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &referral, nil
}

func (r *ReferralRepositoryImpl) FindByReferrerID(referrerID uint) ([]models.Referral, error) {
	var referrals []models.Referral
	err := r.db.Where("referrer_id = ?", referrerID).Order("created_at DESC").Find(&referrals).Error
	return referrals, err
}

func (r *ReferralRepositoryImpl) CountByReferrerID(referrerID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Referral{}).Where("referrer_id = ?", referrerID).Count(&count).Error
	return count, err
}

// CountByReferrerIDSince counts only referrals created this calendar window.
func (r *ReferralRepositoryImpl) CountByReferrerIDSince(referrerID uint, since time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.Referral{}).
		Where("referrer_id = ? AND created_at >= ?", referrerID, since).
		Count(&count).Error
	return count, err
}

// MarkCreditedTx flips a pending referral to credited in the caller's tx.
func (r *ReferralRepositoryImpl) MarkCreditedTx(tx *gorm.DB, referralID uint) (int64, error) {
	now := time.Now()
	res := tx.Model(&models.Referral{}).
		Where("id = ? AND status = ?", referralID, "pending").
		Updates(map[string]interface{}{"status": "credited", "credited_at": now})
	return res.RowsAffected, res.Error
}
