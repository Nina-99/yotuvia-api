package service

import "github.com/Nina-99/yotuvia-api/models"

type ReferralService interface {
	CreateReferralByCode(referrerID uint, code string) error
	ProcessReferralReward(referredID uint) error
	// ProcessPendingReferrals retries every pending referral; inactive referred
	// centers are skipped until they activate.
	ProcessPendingReferrals() (int, error)
	GetReferrals(beautyCenterID uint) ([]models.Referral, error)
	GetReferralInfo(beautyCenterID uint) (code string, count int64, err error)
}
