package repository

import "github.com/Nina-99/yotuvia-api/models"

type ReferralRepository interface {
	Create(referral *models.Referral) error
	Update(referral *models.Referral) error
	FindByID(id uint) (*models.Referral, error)
	FindByReferredID(referredID uint) (*models.Referral, error)
	FindByReferrerID(referrerID uint) ([]models.Referral, error)
	CountByReferrerID(referrerID uint) (int64, error)
}
