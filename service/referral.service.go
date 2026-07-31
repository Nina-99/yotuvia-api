package service

import "github.com/Nina-99/yottu-api/models"

type ReferralService interface {
	CreateReferralByCode(referrerID uint, code string) error
	ProcessReferralReward(referredID uint) error
	GetReferrals(beautyCenterID uint) ([]models.Referral, error)
	GetReferralInfo(beautyCenterID uint) (code string, count int64, err error)
}
