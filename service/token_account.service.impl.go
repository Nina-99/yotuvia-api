package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
)

type TokenAccountServiceImpl struct {
	accountRepo         repository.TokenAccountRepository
	topUpRepo           repository.TokenTopUpRepository
	centerRepo          repository.BeautyCenterRepository
	missionService      TokenMissionService
	notificationService NotificationService
}

func NewTokenAccountServiceImpl(
	accountRepo repository.TokenAccountRepository,
	topUpRepo repository.TokenTopUpRepository,
	centerRepo repository.BeautyCenterRepository,
	missionService TokenMissionService,
	notificationService NotificationService,
) TokenAccountService {
	return &TokenAccountServiceImpl{
		accountRepo:         accountRepo,
		topUpRepo:           topUpRepo,
		centerRepo:          centerRepo,
		missionService:      missionService,
		notificationService: notificationService,
	}
}

func (s *TokenAccountServiceImpl) GetOrCreate(beautyCenterID uint, currency string) (*models.TokenAccount, error) {
	account, err := s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return nil, err
	}
	if account != nil {
		return account, nil
	}

	account = &models.TokenAccount{
		BeautyCenterID: beautyCenterID,
		Balance:        0,
		TotalPurchased: 0,
		TotalConsumed:  0,
		Currency:       currency,
	}
	if err := s.accountRepo.Create(account); err != nil {
		return nil, err
	}
	return account, nil
}

func (s *TokenAccountServiceImpl) GetBalance(beautyCenterID uint) (balance, overdraftLimit int64, currency string, err error) {
	account, err := s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return 0, 0, "", err
	}
	if account == nil {
		// Auto-create for legacy centers registered before token system
		center, cerr := s.centerRepo.FindByID(beautyCenterID)
		if cerr != nil || center == nil {
			return 0, 0, "", errors.New("beauty center not found")
		}
		account, err = s.GetOrCreate(beautyCenterID, center.Currency)
		if err != nil {
			return 0, 0, "", err
		}
	}

	overdraftLimit = utils.ToMilli(s.computeOverdraft(beautyCenterID))
	return account.Balance, overdraftLimit, account.Currency, nil
}

func (s *TokenAccountServiceImpl) computeOverdraft(beautyCenterID uint) float64 {
	account, err := s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil || account == nil {
		return 5 // default for new centers
	}

	// Count top-ups
	topups, err := s.topUpRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return 5
	}

	approvedCount := 0
	for _, t := range topups {
		if t.Status == "approved" {
			approvedCount++
		}
	}

	createdAt := account.CreatedAt
	daysActive := int(time.Since(createdAt).Hours() / 24)

	// Premium: 3+ approved top-ups OR 500+ purchased (500 YOTTUs = 500_000 milli)
	if approvedCount >= 3 || account.TotalPurchased >= 500_000 {
		utils.Log.Debugf("[OVERDRAFT] Center %d — Premium tier (-30)", beautyCenterID)
		return 30
	}

	// Active: 30+ days active
	if daysActive >= 30 {
		utils.Log.Debugf("[OVERDRAFT] Center %d — Active tier (-15)", beautyCenterID)
		return 15
	}

	// New: default
	utils.Log.Debugf("[OVERDRAFT] Center %d — New tier (-5)", beautyCenterID)
	return 5
}

func (s *TokenAccountServiceImpl) CanConsume(beautyCenterID uint, amountMilli int64) (bool, error) {
	account, err := s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return false, err
	}
	if account == nil {
		return false, errors.New("token account not found")
	}

	overdraft := s.computeOverdraft(beautyCenterID)
	available := account.Balance + utils.ToMilli(overdraft)

	return available >= amountMilli, nil
}

func (s *TokenAccountServiceImpl) Consume(beautyCenterID uint, amountMilli int64, referenceID uint, description string) error {
	account, err := s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return err
	}
	if account == nil {
		return errors.New("token account not found")
	}

	var refPtr *uint
	if referenceID != 0 {
		ref := referenceID
		refPtr = &ref
	}

	updated, err := s.accountRepo.Consume(context.Background(), account.ID, amountMilli, "consumption", description, refPtr)
	if err != nil {
		return err
	}

	utils.Log.Infof("[TOKEN] Center %d consumed %.2f YOTTUs. Balance: %.2f",
		beautyCenterID, utils.FromMilli(amountMilli), utils.FromMilli(updated.Balance))
	return nil
}

func (s *TokenAccountServiceImpl) AddTokens(beautyCenterID uint, amountMilli int64, txType string, referenceID uint, description string) error {
	account, err := s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return err
	}
	if account == nil {
		return errors.New("token account not found")
	}

	var refPtr *uint
	if referenceID != 0 {
		ref := referenceID
		refPtr = &ref
	}

	updated, err := s.accountRepo.AddTokens(context.Background(), account.ID, amountMilli, txType, description, refPtr)
	if err != nil {
		return err
	}

	utils.Log.Infof("[TOKEN] Center %d received %.2f YOTTUs (type=%s). Balance: %.2f",
		beautyCenterID, utils.FromMilli(amountMilli), txType, utils.FromMilli(updated.Balance))
	return nil
}

func (s *TokenAccountServiceImpl) ProcessTopUp(topUpID, adminID uint, status string) error {
	topup, err := s.topUpRepo.FindByID(topUpID)
	if err != nil {
		return err
	}
	if topup == nil {
		return errors.New("top-up request not found")
	}
	if topup.Status != "pending" {
		return errors.New("top-up already processed")
	}

	// D5 authz: the admin must own the top-up's center. The role gate on the
	// route guarantees the role; here the service re-proves the claim so direct
	// callers cannot bypass ownership (mirrors CreateAdminReservation).
	if topup.BeautyCenterID != adminID {
		return apperrors.ErrForbidden
	}

	if status == "rejected" {
		topup.Status = "rejected"
		topup.ValidatedBy = &adminID
		now := time.Now()
		topup.ValidatedAt = &now
		if err := s.topUpRepo.Update(topup); err != nil {
			return err
		}
		// Best-effort trigger (design D4): the center is told its top-up was
		// rejected. Never fails the business write.
		_ = s.notificationService.CreateForCenter(topup.BeautyCenterID, "topup_rejected",
			"Recarga rechazada", "Tu solicitud de recarga fue rechazada",
			fmt.Sprintf(`{"topup_id":%d}`, topup.ID))
		return nil
	}

	// approved — credit exactly tokens_received + bonus_tokens; historical grace
	// rows are read-only and never deducted here.
	totalTokens := topup.TokensReceived + topup.BonusTokens

	topup.Status = "approved"
	topup.ValidatedBy = &adminID
	now := time.Now()
	topup.ValidatedAt = &now

	if err := s.topUpRepo.Update(topup); err != nil {
		return err
	}

	// AddTokens credits the balance and logs the ledger row atomically.
	if err := s.AddTokens(topup.BeautyCenterID, totalTokens, "topup", topup.ID, "Top-up approved: "+topup.ReceiptURL); err != nil {
		return err
	}

	// Best-effort trigger (design D4): the center is told its top-up was
	// approved, after the persist + credit succeeded. The pending guard above
	// prevents double-send on reprocess.
	_ = s.notificationService.CreateForCenter(topup.BeautyCenterID, "topup_approved",
		"Recarga aprobada", "Tu recarga fue aprobada",
		fmt.Sprintf(`{"topup_id":%d}`, topup.ID))

	utils.Log.Infof("[TOPUP] Center %d top-up approved: %.2f + %.2f bonus = %.2f YOTTUs",
		topup.BeautyCenterID, utils.FromMilli(topup.TokensReceived), utils.FromMilli(topup.BonusTokens), utils.FromMilli(totalTokens))
	return nil
}
