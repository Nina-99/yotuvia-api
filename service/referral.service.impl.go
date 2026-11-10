package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"gorm.io/gorm"
)

// Referral economics and the monthly creation cap (reward values are milli-Ytv).
const (
	referralRewardReferrer int64 = 20_000 // 20 Ytv
	referralRewardReferred int64 = 10_000 // 10 Ytv
	referralMonthlyLimit   int64 = 10
)

type ReferralServiceImpl struct {
	referralRepo    repository.ReferralRepository
	centerRepo      repository.BeautyCenterRepository
	accountSvc      TokenAccountService
	accountRepo     repository.TokenAccountRepository
	marketConfigSvc MarketConfigService
}

func NewReferralServiceImpl(
	referralRepo repository.ReferralRepository,
	centerRepo repository.BeautyCenterRepository,
	accountSvc TokenAccountService,
	accountRepo repository.TokenAccountRepository,
	marketConfigSvc ...MarketConfigService,
) ReferralService {
	var market MarketConfigService
	if len(marketConfigSvc) > 0 {
		market = marketConfigSvc[0]
	}
	return &ReferralServiceImpl{
		referralRepo:    referralRepo,
		centerRepo:      centerRepo,
		accountSvc:      accountSvc,
		accountRepo:     accountRepo,
		marketConfigSvc: market,
	}
}

func (s *ReferralServiceImpl) CreateReferralByCode(enteredByCenterID uint, code string) error {
	// Codes are uppercase; normalize user input so lowercase/space input works.
	code = strings.ToUpper(strings.TrimSpace(code))

	// Resolve code to the referrer (owner of the code)
	referrer, err := s.centerRepo.FindByReferralCode(code)
	if err != nil {
		return err
	}
	if referrer == nil {
		return errors.New("invalid referral code")
	}
	// Only validated centers can refer; fake/unverified centers cannot farm the
	// referral program.
	if referrer.VerifiedAt == nil {
		return errors.New("referrer is not verified")
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

	// Real monthly cap: count only referrals created since the start of the
	// current calendar month (UTC), not the lifetime total.
	count, err := s.referralRepo.CountByReferrerIDSince(referrer.ID, startOfMonthUTC(time.Now()))
	if err != nil {
		return err
	}
	if count >= referralMonthlyLimit {
		return errors.New("referrer's monthly referral limit reached (max 10)")
	}

	// Snapshot the configured rewards for the referrer's currency. Storing them
	// on the row means a later config change never rewrites an in-flight referral.
	rewardReferrer, rewardReferred := referralRewardReferrer, referralRewardReferred
	if s.marketConfigSvc != nil {
		currency := referrer.Currency
		if currency == "" {
			currency = defaultEconomy.Currency
		}
		economy := s.marketConfigSvc.GetEconomyForCurrency(currency)
		if economy.ReferralRewardReferrerMilli > 0 {
			rewardReferrer = economy.ReferralRewardReferrerMilli
		}
		if economy.ReferralRewardReferredMilli > 0 {
			rewardReferred = economy.ReferralRewardReferredMilli
		}
	}

	referral := &models.Referral{
		ReferrerID:     referrer.ID,
		ReferredID:     referredID,
		Status:         "pending",
		RewardReferrer: rewardReferrer,
		RewardReferred: rewardReferred,
	}

	// Entry only records the pending referral. Rewards are credited when the
	// referred center is VERIFIED (ProcessReferralReward from MarkVerified).
	return s.referralRepo.Create(referral)
}

// ProcessReferralReward credits both wallets exactly once, atomically. The
// status guard and the credits share ONE transaction: a failure leaves the
// referral pending and writes no partial ledger rows.
func (s *ReferralServiceImpl) ProcessReferralReward(referredID uint) error {
	referral, err := s.referralRepo.FindByReferredID(referredID)
	if err != nil {
		return err
	}
	if referral == nil || referral.Status != "pending" {
		return nil // not referred, or already credited
	}
	if s.accountRepo == nil {
		return errors.New("referral: token account repository not configured")
	}

	referrerAccount, err := s.accountRepo.FindByBeautyCenterID(referral.ReferrerID)
	if err != nil {
		return err
	}
	if referrerAccount == nil {
		return errors.New("referral: referrer token account not found")
	}
	referredAccount, err := s.accountRepo.FindByBeautyCenterID(referredID)
	if err != nil {
		return err
	}
	if referredAccount == nil {
		return errors.New("referral: referred token account not found")
	}

	refID := referral.ID
	err = s.accountRepo.Transaction(func(tx *gorm.DB) error {
		rows, err := s.referralRepo.MarkCreditedTx(tx, referral.ID)
		if err != nil {
			return err
		}
		if rows == 0 {
			return nil // concurrent processing won: no second credit
		}

		if _, err := s.accountRepo.AddTokensInTx(tx, referrerAccount.ID,
			referral.RewardReferrer, "referral",
			"Referral reward: center "+fmt.Sprint(referredID), &refID); err != nil {
			return err
		}
		if _, err := s.accountRepo.AddTokensInTx(tx, referredAccount.ID,
			referral.RewardReferred, "referral",
			"Welcome bonus: you were referred!", &refID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		utils.Log.Errorf("[REFERRAL] Failed to credit referral %d (left pending): %v", referral.ID, err)
		return err
	}

	utils.Log.Infof("[REFERRAL] Referral %d credited: referrer +%.0f, referred +%.0f Ytv",
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
