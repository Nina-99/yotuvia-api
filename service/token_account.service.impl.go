package service

import (
	"errors"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
)

type TokenAccountServiceImpl struct {
	accountRepo    repository.TokenAccountRepository
	topUpRepo      repository.TokenTopUpRepository
	txRepo         repository.TokenTransactionRepository
	centerRepo     repository.BeautyCenterRepository
	missionService TokenMissionService
}

func NewTokenAccountServiceImpl(
	accountRepo repository.TokenAccountRepository,
	topUpRepo repository.TokenTopUpRepository,
	txRepo repository.TokenTransactionRepository,
	centerRepo repository.BeautyCenterRepository,
	missionService TokenMissionService,
) TokenAccountService {
	return &TokenAccountServiceImpl{
		accountRepo:    accountRepo,
		topUpRepo:      topUpRepo,
		txRepo:         txRepo,
		centerRepo:     centerRepo,
		missionService: missionService,
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

func (s *TokenAccountServiceImpl) GetBalance(beautyCenterID uint) (balance, overdraftLimit float64, currency string, err error) {
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

	overdraftLimit = s.computeOverdraft(beautyCenterID)
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

	// Premium: 3+ approved top-ups OR 500+ purchased
	if approvedCount >= 3 || account.TotalPurchased >= 500 {
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

func (s *TokenAccountServiceImpl) CanConsume(beautyCenterID uint, amount float64) (bool, error) {
	account, err := s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return false, err
	}
	if account == nil {
		return false, errors.New("token account not found")
	}

	overdraft := s.computeOverdraft(beautyCenterID)
	available := account.Balance + overdraft

	return available >= amount, nil
}

func (s *TokenAccountServiceImpl) Consume(beautyCenterID uint, amount float64, referenceID uint, description string) error {
	account, err := s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return err
	}
	if account == nil {
		return errors.New("token account not found")
	}

	overdraft := s.computeOverdraft(beautyCenterID)
	if account.Balance+overdraft < amount {
		return errors.New("insufficient token balance")
	}

	balanceBefore := account.Balance
	account.Balance -= amount
	account.TotalConsumed += amount

	if err := s.accountRepo.Update(account); err != nil {
		// Log grace transaction on failure
		utils.Log.Errorf("[TOKEN GRACE] Failed to update balance for center %d: %v. Logging grace tx.", beautyCenterID, err)
		graceTx := &models.TokenTransaction{
			BeautyCenterID: beautyCenterID,
			Amount:         -amount,
			Type:           "grace",
			ReferenceID:    &referenceID,
			BalanceBefore:  balanceBefore,
			BalanceAfter:   balanceBefore,
			Description:    "GRACE: " + description + " (deduct failed)",
		}
		_ = s.txRepo.Create(graceTx)
		return err
	}

	// Log successful consumption
	tx := &models.TokenTransaction{
		BeautyCenterID: beautyCenterID,
		Amount:         -amount,
		Type:           "consumption",
		ReferenceID:    &referenceID,
		BalanceBefore:  balanceBefore,
		BalanceAfter:   account.Balance,
		Description:    description,
	}
	if err := s.txRepo.Create(tx); err != nil {
		utils.Log.Errorf("[TOKEN] Failed to log transaction for center %d: %v", beautyCenterID, err)
	}

	utils.Log.Infof("[TOKEN] Center %d consumed %.2f YOTTUs. Balance: %.2f", beautyCenterID, amount, account.Balance)
	return nil
}

func (s *TokenAccountServiceImpl) AddTokens(beautyCenterID uint, amount float64, txType string, referenceID uint, description string) error {
	account, err := s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return err
	}
	if account == nil {
		return errors.New("token account not found")
	}

	balanceBefore := account.Balance
	account.Balance += amount
	account.TotalPurchased += amount

	if err := s.accountRepo.Update(account); err != nil {
		return err
	}

	tx := &models.TokenTransaction{
		BeautyCenterID: beautyCenterID,
		Amount:         amount,
		Type:           txType,
		ReferenceID:    &referenceID,
		BalanceBefore:  balanceBefore,
		BalanceAfter:   account.Balance,
		Description:    description,
	}
	if err := s.txRepo.Create(tx); err != nil {
		utils.Log.Errorf("[TOKEN] Failed to log transaction for center %d: %v", beautyCenterID, err)
	}

	utils.Log.Infof("[TOKEN] Center %d received %.2f YOTTUs (type=%s). Balance: %.2f", beautyCenterID, amount, txType, account.Balance)
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

	if status == "rejected" {
		topup.Status = "rejected"
		topup.ValidatedBy = &adminID
		now := time.Now()
		topup.ValidatedAt = &now
		return s.topUpRepo.Update(topup)
	}

	// approved
	totalTokens := topup.TokensReceived + topup.BonusTokens

	// Check for grace debt from previous failed consumptions
	// Deduct any outstanding grace transactions first
	account, err := s.accountRepo.FindByBeautyCenterID(topup.BeautyCenterID)
	if err != nil {
		return err
	}
	if account == nil {
		return errors.New("token account not found")
	}

	topup.Status = "approved"
	topup.ValidatedBy = &adminID
	now := time.Now()
	topup.ValidatedAt = &now

	if err := s.topUpRepo.Update(topup); err != nil {
		return err
	}

	// Add tokens to account (AddTokens will also log the transaction)
	if err := s.AddTokens(topup.BeautyCenterID, totalTokens, "topup", topup.ID, "Top-up approved: "+topup.ReceiptURL); err != nil {
		return err
	}

	utils.Log.Infof("[TOPUP] Center %d top-up approved: %.2f + %.2f bonus = %.2f YOTTUs",
		topup.BeautyCenterID, topup.TokensReceived, topup.BonusTokens, totalTokens)
	return nil
}
