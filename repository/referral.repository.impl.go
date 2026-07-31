package repository

import (
	"github.com/Nina-99/yottu-api/models"
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
