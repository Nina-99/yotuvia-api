package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
)

type ReferralServiceImpl struct {
	referralRepo repository.ReferralRepository
	centerRepo   repository.BeautyCenterRepository
	accountSvc   TokenAccountService
}

func NewReferralServiceImpl(
	referralRepo repository.ReferralRepository,
	centerRepo repository.BeautyCenterRepository,
	accountSvc TokenAccountService,
) ReferralService {
	return &ReferralServiceImpl{
		referralRepo: referralRepo,
		centerRepo:   centerRepo,
		accountSvc:   accountSvc,
	}
}

func (s *ReferralServiceImpl) CreateReferralByCode(enteredByCenterID uint, code string) error {
	// Resolve code to the referrer (owner of the code)
	referrer, err := s.centerRepo.FindByReferralCode(code)
	if err != nil {
		return err
	}
	if referrer == nil {
		return errors.New("invalid referral code")
	}

	// The center entering a code is the referred
	referredID := enteredByCenterID

	if referredID == referrer.ID {
		return errors.New("cannot refer yourself")
	}

	// Check if this center has already been referred
	existing, err := s.referralRepo.FindByReferredID(referredID)
	if err != nil {
		return err
	}
	if existing != nil {
		return errors.New("this center has already been referred")
	}

	// Check monthly limit on the referrer (max 10 outgoing referrals per month)
	count, err := s.referralRepo.CountByReferrerID(referrer.ID)
	if err != nil {
		return err
	}
	if count >= 10 {
		return errors.New("referrer's monthly referral limit reached (max 10)")
	}

	referral := &models.Referral{
		ReferrerID:     referrer.ID,
		ReferredID:     referredID,
		Status:         "pending",
		RewardReferrer: 20000, // 20 YOTTUs in milli
		RewardReferred: 10000, // 10 YOTTUs in milli
	}

	if err := s.referralRepo.Create(referral); err != nil {
		return err
	}

	// Auto-process reward
	return s.ProcessReferralReward(referredID)
}

func (s *ReferralServiceImpl) ProcessReferralReward(referredID uint) error {
	referral, err := s.referralRepo.FindByReferredID(referredID)
	if err != nil {
		return err
	}
	if referral == nil {
		return nil // not referred by anyone
	}
	if referral.Status != "pending" {
		return nil // already processed
	}

	referral.Status = "credited"
	now := time.Now()
	referral.CreditedAt = &now

	if err := s.referralRepo.Update(referral); err != nil {
		return err
	}

	// Reward referrer
	if err := s.accountSvc.AddTokens(referral.ReferrerID, referral.RewardReferrer, "referral", referral.ID, "Referral reward: center "+fmt.Sprint(referredID)); err != nil {
		utils.Log.Errorf("[REFERRAL] Failed to reward referrer %d: %v", referral.ReferrerID, err)
	}

	// Reward referred
	if err := s.accountSvc.AddTokens(referredID, referral.RewardReferred, "referral", referral.ID, "Welcome bonus: you were referred!"); err != nil {
		utils.Log.Errorf("[REFERRAL] Failed to reward referred %d: %v", referredID, err)
	}

	utils.Log.Infof("[REFERRAL] Referral %d credited: referrer +%.0f, referred +%.0f YOTTUs",
		referral.ID, utils.FromMilli(referral.RewardReferrer), utils.FromMilli(referral.RewardReferred))
	return nil
}

func (s *ReferralServiceImpl) GetReferrals(beautyCenterID uint) ([]models.Referral, error) {
	return s.referralRepo.FindByReferrerID(beautyCenterID)
}

func (s *ReferralServiceImpl) GetReferralInfo(beautyCenterID uint) (code string, count int64, err error) {
	center, err := s.centerRepo.FindByID(beautyCenterID)
	if err != nil {
		return "", 0, err
	}
	if center == nil {
		return "", 0, errors.New("beauty center not found")
	}
	count, err = s.referralRepo.CountByReferrerID(beautyCenterID)
	if err != nil {
		return "", 0, err
	}
	return center.ReferralCode, count, nil
}
