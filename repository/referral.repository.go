package repository

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type ReferralRepository interface {
	Create(referral *models.Referral) error
	Update(referral *models.Referral) error
	FindByID(id uint) (*models.Referral, error)
	FindByReferredID(referredID uint) (*models.Referral, error)
	FindByReferrerID(referrerID uint) ([]models.Referral, error)
	CountByReferrerID(referrerID uint) (int64, error)
	// CountByReferrerIDSince counts referrals created at/after `since` (UTC), so
	// the monthly cap is a real calendar window instead of a lifetime count.
	CountByReferrerIDSince(referrerID uint, since time.Time) (int64, error)
	// ExpirePendingBefore marks pending referrals older than `before` as
	// expired, freeing the monthly slot. Returns the number of rows affected.
	ExpirePendingBefore(before time.Time) (int64, error)
	// MarkCreditedTx flips a pending referral to credited inside the caller's
	// transaction, guarded by status = pending. Rows affected zero means another
	// transaction already processed it.
	MarkCreditedTx(tx *gorm.DB, referralID uint) (int64, error)
}
