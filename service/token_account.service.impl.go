package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"gorm.io/gorm"
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

func (s *TokenAccountServiceImpl) GetBalance(beautyCenterID uint) (balance, reserved, overdraftLimit int64, currency string, err error) {
	account, err := s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return 0, 0, 0, "", err
	}
	if account == nil {
		// Auto-create for legacy centers registered before token system
		center, cerr := s.centerRepo.FindByID(beautyCenterID)
		if cerr != nil || center == nil {
			return 0, 0, 0, "", errors.New("beauty center not found")
		}
		account, err = s.GetOrCreate(beautyCenterID, center.Currency)
		if err != nil {
			return 0, 0, 0, "", err
		}
	}

	overdraftLimit = utils.ToMilli(s.computeOverdraft(beautyCenterID))
	return account.Balance, account.ReservedMilli, overdraftLimit, account.Currency, nil
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

	// Premium: 3+ approved top-ups OR 500+ purchased (500 Ytv = 500_000 milli)
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
	// Holds are not spendable: available excludes the reserved portion.
	available := account.Balance - account.ReservedMilli
	available += utils.ToMilli(overdraft)

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

	overdraftMilli := utils.ToMilli(s.computeOverdraft(beautyCenterID))
	var refPtr *uint
	if referenceID != 0 {
		ref := referenceID
		refPtr = &ref
	}

	err = s.accountRepo.Transaction(func(tx *gorm.DB) error {
		return s.accountRepo.DebitInTx(tx, account.ID, amountMilli, overdraftMilli, "consumption", description, refPtr)
	})
	if err != nil {
		return err
	}

	utils.Log.Infof("[TOKEN] Center %d consumed %.2f Ytv (with overdraft).",
		beautyCenterID, utils.FromMilli(amountMilli))
	return nil
}

// Reserve holds funds for a prepaid purchase (e.g. a boost campaign). It moves
// no money: the balance is untouched and only ReservedMilli grows.
func (s *TokenAccountServiceImpl) Reserve(beautyCenterID uint, amountMilli int64, capPct float64) error {
	account, err := s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return err
	}
	if account == nil {
		return errors.New("token account not found")
	}
	if _, err := s.accountRepo.Reserve(context.Background(), account.ID, amountMilli, capPct); err != nil {
		return err
	}
	utils.Log.Infof("[TOKEN] Center %d reserved %.2f Ytv.", beautyCenterID, utils.FromMilli(amountMilli))
	return nil
}

// SettleReserved spends from an existing hold and records the ledger row. It
// can never take more than is held and never falls back to overdraft.
func (s *TokenAccountServiceImpl) SettleReserved(beautyCenterID uint, amountMilli int64, txType string, referenceID uint, description string) error {
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

	if _, err := s.accountRepo.SettleReserved(context.Background(), account.ID, amountMilli, txType, description, refPtr); err != nil {
		return err
	}
	utils.Log.Infof("[TOKEN] Center %d settled %.2f Ytv from hold (type=%s).",
		beautyCenterID, utils.FromMilli(amountMilli), txType)
	return nil
}

// ReleaseReserved returns held funds to available without moving money.
func (s *TokenAccountServiceImpl) ReleaseReserved(beautyCenterID uint, amountMilli int64) error {
	account, err := s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return err
	}
	if account == nil {
		return errors.New("token account not found")
	}
	if _, err := s.accountRepo.ReleaseReserved(context.Background(), account.ID, amountMilli); err != nil {
		return err
	}
	utils.Log.Infof("[TOKEN] Center %d released %.2f Ytv from hold.", beautyCenterID, utils.FromMilli(amountMilli))
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

	utils.Log.Infof("[TOKEN] Center %d received %.2f Ytv (type=%s). Balance: %.2f",
		beautyCenterID, utils.FromMilli(amountMilli), txType, utils.FromMilli(updated.Balance))
	return nil
}

// loadPendingTopUp resolves a top-up and applies the shared "not found / already
// processed" guards, so every caller — centre-scoped or platform-scoped —
// answers with the exact same messages.
func (s *TokenAccountServiceImpl) loadPendingTopUp(topUpID uint) (*models.TokenTopUp, error) {
	topup, err := s.topUpRepo.FindByID(topUpID)
	if err != nil {
		return nil, err
	}
	if topup == nil {
		return nil, errors.New("top-up request not found")
	}
	if topup.Status != "pending" {
		return nil, errors.New("top-up already processed")
	}
	return topup, nil
}

// ProcessTopUp is the centre-scoped decision path. D5 authz: the admin must own
// the top-up's center. The role gate on the route guarantees the role; here the
// service re-proves the claim so direct callers cannot bypass ownership
// (mirrors CreateAdminReservation).
func (s *TokenAccountServiceImpl) ProcessTopUp(topUpID, adminID uint, status string) error {
	topup, err := s.loadPendingTopUp(topUpID)
	if err != nil {
		return err
	}
	if topup.BeautyCenterID != adminID {
		return apperrors.ErrForbidden
	}
	return s.applyTopUpDecision(topup, adminID, status)
}

// ProcessTopUpAsPlatform is the platform-scoped decision path: a super_admin may
// decide ANY centre's top-up, so the ownership check that guards the
// centre-scoped path is deliberately absent. Everything else — the guards above,
// the approve/reject semantics, the credit and the notifications — is the SAME
// code, so the two paths cannot drift into two contracts. adminID is the
// platform admin's own id and is what gets recorded as the validator.
func (s *TokenAccountServiceImpl) ProcessTopUpAsPlatform(topUpID, adminID uint, status string) error {
	topup, err := s.loadPendingTopUp(topUpID)
	if err != nil {
		return err
	}
	return s.applyTopUpDecision(topup, adminID, status)
}

// applyTopUpDecision performs the approved/rejected write, the balance credit and
// the best-effort notification. It assumes the top-up is already known to be
// pending.
func (s *TokenAccountServiceImpl) applyTopUpDecision(topup *models.TokenTopUp, adminID uint, status string) error {
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

	utils.Log.Infof("[TOPUP] Center %d top-up approved: %.2f + %.2f bonus = %.2f Ytv",
		topup.BeautyCenterID, utils.FromMilli(topup.TokensReceived), utils.FromMilli(topup.BonusTokens), utils.FromMilli(totalTokens))
	return nil
}

func (s *TokenAccountServiceImpl) FindMyTopUps(centerID uint, page, limit int, status *string) ([]models.TokenTopUp, int64, error) {
	var statusStr string
	if status != nil {
		statusStr = *status
	}
	return s.topUpRepo.FindByCenterIDWithStatus(centerID, page, limit, statusStr)
}
