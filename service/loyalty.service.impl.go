package service

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/go-playground/validator/v10"
)

type LoyaltyServiceImpl struct {
	repo           repository.LoyaltyRepository
	centerRepo     repository.BeautyCenterRepository
	missionService TokenMissionService
	Validate       *validator.Validate
}

func NewLoyaltyService(repo repository.LoyaltyRepository, centerRepo repository.BeautyCenterRepository, missionService TokenMissionService, validate *validator.Validate) LoyaltyService {
	return &LoyaltyServiceImpl{
		repo:           repo,
		centerRepo:     centerRepo,
		missionService: missionService,
		Validate:       validate,
	}
}

func (s *LoyaltyServiceImpl) CreateProgram(req request.CreateLoyaltyProgrmaRequest) (response.LoyaltyProgramResponse, error) {
	if err := s.Validate.Struct(req); err != nil {
		return response.LoyaltyProgramResponse{}, err
	}

	// Check if there is already an active program for this service at this center
	if req.ServiceID != nil {
		existing, err := s.repo.FindProgramByService(req.BeautyCenterID, *req.ServiceID)
		if err != nil {
			return response.LoyaltyProgramResponse{}, err
		}
		if existing != nil {
			return response.LoyaltyProgramResponse{}, errors.New("A loyalty program exists for this service")
		}
	}

	program := models.LoyaltyProgram{
		BeautyCenterID:  req.BeautyCenterID,
		ServiceID:       req.ServiceID,
		RewardServiceID: req.RewardServiceID,
		RequiredSteps:   req.RequiredSteps,
		RewardType:      req.RewardType,
		DiscountValue:   req.DiscountValue,
		DurationDays:    req.DurationDays,
		IsActive:        true,
	}

	// Persist the program — it was previously assembled but never saved
	// (drive-by fix required by the stamp_activated trigger: there is nothing
	// to activate without a row).
	if err := s.repo.CreateProgram(&program); err != nil {
		return response.LoyaltyProgramResponse{}, err
	}

	utils.Log.Info("[Loyalty] New program created for BeautyCenteer %d on Service %d", req.BeautyCenterID, req.ServiceID)

	// Best-effort mission trigger (feat-token-missions D6): activating a stamp
	// card program counts as stamp_activated. Never fails the primary write.
	if s.missionService != nil {
		if err := s.missionService.TriggerCheck(req.BeautyCenterID, "stamp_activated"); err != nil {
			utils.Log.Errorf("[MISSION] stamp_activated trigger failed for center %d: %v",
				req.BeautyCenterID, err)
		}
	}

	return response.LoyaltyProgramResponse{
		ID:              program.ID,
		BeautyCenterID:  program.BeautyCenterID,
		ServiceID:       program.ServiceID,
		RewardServiceID: program.RewardServiceID,
		RequiredSteps:   program.RequiredSteps,
		RewardType:      program.RewardType,
		DiscountValue:   program.DiscountValue,
		DurationDays:    program.DurationDays,
		IsActive:        program.IsActive,
		CreatedAt:       program.CreatedAt,
	}, nil
}

// deriveExpiresAt computes the card expiry from a base time + the program's
// nullable duration policy. Nil policy → nil (no expiry). Slice 1 derives and
// persists WITHOUT enforcement.
func deriveExpiresAt(base time.Time, durationDays *int) *time.Time {
	if durationDays == nil || *durationDays <= 0 {
		return nil
	}
	t := base.AddDate(0, 0, *durationDays)
	return &t
}

// cardExpired reports whether a card's derived expiry has passed. Nil expiry
// (no policy) never expires. Slice 2 enforcement: an expired card neither
// accrues stamps nor redeems its reward, and is excluded from pending reads.
func cardExpired(card *models.CustomerPunchCard, now time.Time) bool {
	if card == nil || card.ExpiresAt == nil {
		return false
	}
	return now.After(*card.ExpiresAt)
}

// isDuplicateKey reports whether err is a UNIQUE-constraint violation across
// postgres (pgx) and sqlite drivers, so the double-punch guard degrades to a
// no-op success instead of a 500 under concurrent scans.
func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "unique_constraint") ||
		strings.Contains(msg, "23505")
}
func (s *LoyaltyServiceImpl) AddPunch(reservationID, clientID, beautyCenterID, serviceID uint) error {
	// 1. We check if there is an active loyalty program
	program, err := s.repo.FindProgramByService(beautyCenterID, serviceID)
	if err != nil {
		return err
	}
	if program == nil {
		return nil
	}

	// 2. We look for the customer's active for this promotion
	card, err := s.repo.FindActiveCard(clientID, program.ID)
	if err != nil {
		return err
	}
	// Slice 2: an EXPIRED active card must not accrue more stamps. Retire it
	// (status "expired") so the client restarts a fresh cycle below, instead of
	// silently dying forever or accruing on a dead card.
	if card != nil && cardExpired(card, time.Now()) {
		card.Status = "expired"
		if err := s.repo.UpdatePunchCard(card); err != nil {
			return err
		}
		card = nil
	}
	// 3. If there is no active, we create one
	if card == nil {
		card = &models.CustomerPunchCard{
			ClientID:         clientID,
			LoyaltyProgramID: program.ID,
			CurrentSteps:     0,
			Status:           "active",
			ExpiresAt:        deriveExpiresAt(time.Now(), program.DurationDays),
		}
		if err := s.repo.CreatePunchCard(card); err != nil {
			return err
		}
	}

	// Double-punch guard: the (reservation, card) UNIQUE pair means a
	// concurrent re-scan (or retry) of the same reservation must not mint a
	// second stamp. Procedural check first; the DB constraint + duplicate
	// tolerance below covers the race window.
	if existing, err := s.repo.FindPunchHistoryByReservationCard(reservationID, card.ID); err != nil {
		return err
	} else if existing != nil {
		utils.Log.Infof("[LOYALTY] Duplicate punch ignored for reservation %d card %d", reservationID, card.ID)
		return nil
	}

	card.CurrentSteps++

	// 4. Audit the stamp history
	history := models.PunchHistory{
		CustomerPunchCardID: card.ID,
		ReservationID:       reservationID,
	}
	if err := s.repo.CreatePunchHistory(&history); err != nil {
		if isDuplicateKey(err) {
			utils.Log.Infof("[LOYALTY] Concurrent duplicate punch absorbed for reservation %d card %d", reservationID, card.ID)
			return nil
		}
		return err
	}

	// 5. Verify if the card was completed
	if card.CurrentSteps >= program.RequiredSteps {
		card.Status = "completed"
		utils.Log.Infof("[LOYALTY] Client %d completed Loyalty Program %d! Reward earned.", clientID, program.ID)
	}

	if err := s.repo.UpdatePunchCard(card); err != nil {
		return err
	}

	utils.Log.Infof("[LOYALTY] Sello sumado a Client %d en Program %d. Pasos: %d/%d", clientID, program.ID, card.CurrentSteps, program.RequiredSteps)
	return nil
}

func (s *LoyaltyServiceImpl) GetClientsCards(clientID, actorClientID uint, role string) ([]response.PunchCardResponse, error) {
	// Ownership: a `client` may read only its own cards. beauty_center and
	// personnel keep inspecting any client's cards — that is a legitimate
	// staff-facing view, not an IDOR.
	if role == "client" && actorClientID != clientID {
		return nil, apperrors.ErrForbidden
	}

	cards, err := s.repo.FindCardsByClient(clientID)
	if err != nil {
		return nil, err
	}

	var res []response.PunchCardResponse
	for _, c := range cards {
		res = append(res, response.PunchCardResponse{
			ID:           c.ID,
			CurrentSteps: c.CurrentSteps,
			Status:       c.Status,
			ExpiresAt:    c.ExpiresAt,
			LoyaltyProgram: response.LoyaltyProgramResponse{
				ID:             c.LoyaltyProgram.ID,
				BeautyCenterID: c.LoyaltyProgram.BeautyCenterID,
				ServiceID:      c.LoyaltyProgram.ServiceID,
				ServiceName:    c.LoyaltyProgram.Service.Name,
				RequiredSteps:  c.LoyaltyProgram.RequiredSteps,
				RewardType:     c.LoyaltyProgram.RewardType,
				DiscountValue:  c.LoyaltyProgram.DiscountValue,
				DurationDays:   c.LoyaltyProgram.DurationDays,
				IsActive:       c.LoyaltyProgram.IsActive,
				CreatedAt:      c.LoyaltyProgram.CreatedAt,
			},
		})
	}
	s.enrichCardsCenter(res, cards)

	return res, nil
}

// enrichCardsCenter fills BeautyCenterID/Name on card + program responses from
// the REAL center rows (never hardcoded). Missing centers degrade to zero/empty
// rather than failing the read.
func (s *LoyaltyServiceImpl) enrichCardsCenter(res []response.PunchCardResponse, cards []models.CustomerPunchCard) {
	if s.centerRepo == nil {
		return
	}
	names := map[uint]string{}
	for i, c := range cards {
		bcID := c.LoyaltyProgram.BeautyCenterID
		res[i].BeautyCenterID = bcID
		res[i].LoyaltyProgram.BeautyCenterID = bcID
		name, ok := names[bcID]
		if !ok {
			if center, err := s.centerRepo.FindByID(bcID); err == nil && center != nil {
				name = center.Name
			}
			names[bcID] = name
		}
		res[i].BeautyCenterName = name
		res[i].LoyaltyProgram.BeautyCenterName = name
	}
}

// rewardDescription renders the human-readable reward summary shared by the
// pending, public, and verify-qr payloads.
func rewardDescription(rewardType, serviceName string, discountValue float64) string {
	switch rewardType {
	case "free":
		if serviceName != "" {
			return "Free " + serviceName
		}
		return "Free service"
	case "percentage_discount":
		return "Discount of " + strconv.FormatFloat(discountValue, 'f', -1, 64) + "%"
	default:
		return rewardType
	}
}

// checkCardOwnership enforces the client-owns-cards contract shared by the
// client loyalty reads.
func checkCardOwnership(clientID, actorClientID uint, role string) error {
	if role == "client" && actorClientID != clientID {
		return apperrors.ErrForbidden
	}
	return nil
}

// GetHistory returns the client's punch timeline, newest first.
func (s *LoyaltyServiceImpl) GetHistory(clientID, actorClientID uint, role string) ([]response.PunchHistoryItemResponse, error) {
	if err := checkCardOwnership(clientID, actorClientID, role); err != nil {
		return nil, err
	}
	rows, err := s.repo.FindPunchHistoryByClient(clientID)
	if err != nil {
		return nil, err
	}
	res := make([]response.PunchHistoryItemResponse, 0, len(rows))
	for _, r := range rows {
		item := response.PunchHistoryItemResponse{
			ID:        r.ID,
			CardID:    r.CustomerPunchCardID,
			ProgramID: r.CustomerPunchCard.LoyaltyProgramID,
			CreatedAt: r.CreatedAt,
		}
		if r.CustomerPunchCard.LoyaltyProgram.ID != 0 {
			item.Required = r.CustomerPunchCard.LoyaltyProgram.RequiredSteps
			item.ServiceName = r.CustomerPunchCard.LoyaltyProgram.Service.Name
		}
		res = append(res, item)
	}
	// Stamps = position of this punch within its card (1-based order by time).
	// Cheap derivation: count rows per card in timeline order (oldest first).
	seen := map[uint]int{}
	for i := len(res) - 1; i >= 0; i-- {
		seen[res[i].CardID]++
		res[i].Stamps = seen[res[i].CardID]
	}
	return res, nil
}

// GetPendingRewards returns completed-but-unredeemed cards for the client.
func (s *LoyaltyServiceImpl) GetPendingRewards(clientID, actorClientID uint, role string) ([]response.PendingRewardResponse, error) {
	if err := checkCardOwnership(clientID, actorClientID, role); err != nil {
		return nil, err
	}
	cards, err := s.repo.FindCompletedCardsByClient(clientID)
	if err != nil {
		return nil, err
	}
	res := make([]response.PendingRewardResponse, 0, len(cards))
	for _, c := range cards {
		// Slice 2: an expired completed card can no longer be redeemed.
		if cardExpired(&c, time.Now()) {
			continue
		}
		redemption, err := s.repo.FindRedemptionByCard(c.ID)
		if err != nil {
			return nil, err
		}
		if redemption != nil {
			continue
		}
		res = append(res, response.PendingRewardResponse{
			CardID:            c.ID,
			ProgramID:         c.LoyaltyProgramID,
			ServiceName:       c.LoyaltyProgram.Service.Name,
			RewardDescription: rewardDescription(c.LoyaltyProgram.RewardType, c.LoyaltyProgram.Service.Name, c.LoyaltyProgram.DiscountValue),
			RewardType:        c.LoyaltyProgram.RewardType,
			DiscountValue:     c.LoyaltyProgram.DiscountValue,
		})
	}
	return res, nil
}

// GetPublicPrograms returns anonymous-safe active program summaries.
func (s *LoyaltyServiceImpl) GetPublicPrograms(beautyCenterID uint) ([]response.PublicProgramResponse, error) {
	programs, err := s.repo.FindPublicProgramsByCenter(beautyCenterID)
	if err != nil {
		return nil, err
	}
	res := make([]response.PublicProgramResponse, 0, len(programs))
	for _, p := range programs {
		res = append(res, response.PublicProgramResponse{
			ID:                p.ID,
			ServiceName:       p.Service.Name,
			RequiredSteps:     p.RequiredSteps,
			RewardType:        p.RewardType,
			RewardDescription: rewardDescription(p.RewardType, p.Service.Name, p.DiscountValue),
			DiscountValue:     p.DiscountValue,
			IsActive:          p.IsActive,
		})
	}
	return res, nil
}

// HasPendingRedemption checks if the client has a completed, unredeemed punch
// card for the given beauty center + service.
func (s *LoyaltyServiceImpl) HasPendingRedemption(clientID, serviceID, beautyCenterID uint) bool {
	program, err := s.repo.FindProgramByService(beautyCenterID, serviceID)
	if err != nil || program == nil {
		return false
	}
	card, err := s.repo.FindCompletedCard(clientID, program.ID)
	if err != nil || card == nil {
		return false
	}
	// Slice 2: an expired card is not redeemable.
	if cardExpired(card, time.Now()) {
		return false
	}
	// Ensure it hasn't been redeemed yet
	redemption, err := s.repo.FindRedemptionByCard(card.ID)
	if err != nil {
		return false
	}
	return redemption == nil
}

// CalculateRedemption computes the discount amount without committing anything.
// Returns (discount, cardID, nil). The caller must later call
// FinalizeRedemption to persist the record.
func (s *LoyaltyServiceImpl) CalculateRedemption(clientID, serviceID, beautyCenterID uint) (float64, uint, error) {
	program, err := s.repo.FindProgramByService(beautyCenterID, serviceID)
	if err != nil || program == nil {
		return 0, 0, nil
	}

	card, err := s.repo.FindCompletedCard(clientID, program.ID)
	if err != nil || card == nil {
		return 0, 0, nil
	}

	// Slice 2: an expired card redeems nothing.
	if cardExpired(card, time.Now()) {
		return 0, 0, nil
	}

	// Ensure it hasn't been redeemed yet
	existing, err := s.repo.FindRedemptionByCard(card.ID)
	if err != nil {
		return 0, 0, err
	}
	if existing != nil {
		return 0, 0, nil
	}

	// Calculate discount based on reward type.
	// If a reward service is specified, use its price; otherwise fall back to
	// the accumulated service's price.
	var rewardPrice float64
	if program.RewardServiceID != nil && program.RewardService.Price > 0 {
		rewardPrice = program.RewardService.Price
	} else {
		rewardPrice = program.Service.Price
	}

	var discount float64
	switch program.RewardType {
	case "free":
		discount = rewardPrice
	case "percentage_discount":
		discount = rewardPrice * program.DiscountValue / 100
	}

	return discount, card.ID, nil
}

// FinalizeRedemption records the redemption, resets the card, and links it to
// the given reservation. Called after reservation creation succeeds.
func (s *LoyaltyServiceImpl) FinalizeRedemption(cardID, reservationID uint) error {
	// Get the card details
	cardPtr, err := s.repo.FindCompletedCardByCardID(cardID)
	if err != nil || cardPtr == nil {
		return err
	}

	// Slice 2 defense: never finalize a redemption on an expired card.
	if cardExpired(cardPtr, time.Now()) {
		return nil
	}

	// Get the program to calculate the discount
	var programPtr *models.LoyaltyProgram
	if cardPtr.LoyaltyProgram.ServiceID != nil {
		programPtr, err = s.repo.FindProgramByService(cardPtr.LoyaltyProgram.BeautyCenterID, *cardPtr.LoyaltyProgram.ServiceID)
	} else {
		// Program applies to all services — load by ID directly
		programPtr, err = s.repo.FindProgramByID(cardPtr.LoyaltyProgramID)
	}
	if err != nil || programPtr == nil {
		return err
	}

	// Calculate reward price: use reward service if set, else accumulated service
	var rewardPrice float64
	if programPtr.RewardServiceID != nil && programPtr.RewardService.Price > 0 {
		rewardPrice = programPtr.RewardService.Price
	} else {
		rewardPrice = programPtr.Service.Price
	}

	var discount float64
	switch programPtr.RewardType {
	case "free":
		discount = rewardPrice
	case "percentage_discount":
		discount = rewardPrice * programPtr.DiscountValue / 100
	}

	// Record the redemption
	redemption := models.LoyaltyRedemption{
		CustomerPunchCardID: cardID,
		ReservationID:       reservationID,
		DiscountApplied:     discount,
	}
	if err := s.repo.CreateRedemption(&redemption); err != nil {
		return err
	}

	// Reset the card for next cycle (expires_at re-derived from the program's
	// duration policy; nil policy = no expiry).
	cardPtr.CurrentSteps = 0
	cardPtr.Status = "active"
	cardPtr.ExpiresAt = deriveExpiresAt(time.Now(), programPtr.DurationDays)
	if err := s.repo.UpdatePunchCard(cardPtr); err != nil {
		return err
	}

	utils.Log.Infof("[LOYALTY] Card %d redeemed for reservation %d. Discount: %.2f. Card reset.", cardID, reservationID, discount)
	return nil
}

// FindRedemptionByReservation checks if a reservation already has a loyalty
// redemption (idempotency guard for the reservation flow).
func (s *LoyaltyServiceImpl) FindRedemptionByReservation(reservationID uint) bool {
	redemption, err := s.repo.FindRedemptionByReservation(reservationID)
	if err != nil {
		return false
	}
	return redemption != nil
}

// ── Center program management ────────────────────────────────

func (s *LoyaltyServiceImpl) FindProgramsByCenter(beautyCenterID, actorCenterID uint) ([]response.LoyaltyProgramResponse, error) {
	// Reject a :centerId path value that does not match the caller's claim.
	if actorCenterID == 0 || actorCenterID != beautyCenterID {
		return nil, apperrors.ErrForbidden
	}

	programs, err := s.repo.FindProgramsByCenter(beautyCenterID)
	if err != nil {
		return nil, err
	}
	var res []response.LoyaltyProgramResponse
	for _, p := range programs {
		resp := response.LoyaltyProgramResponse{
			ID:              p.ID,
			ServiceID:       p.ServiceID,
			RewardServiceID: p.RewardServiceID,
			RequiredSteps:   p.RequiredSteps,
			RewardType:      p.RewardType,
			DiscountValue:   p.DiscountValue,
			IsActive:        p.IsActive,
			CreatedAt:       p.CreatedAt,
		}
		if p.Service.Name != "" {
			resp.ServiceName = p.Service.Name
		}
		if p.RewardService.Name != "" {
			resp.RewardServiceName = p.RewardService.Name
		}
		res = append(res, resp)
	}
	return res, nil
}

func (s *LoyaltyServiceImpl) UpdateProgram(id, actorCenterID uint, req request.UpdateLoyaltyProgramRequest) (response.LoyaltyProgramResponse, error) {
	program, err := s.repo.FindProgramByID(id)
	if err != nil {
		return response.LoyaltyProgramResponse{}, err
	}
	if program == nil {
		return response.LoyaltyProgramResponse{}, apperrors.ErrNotFound
	}
	// Ownership: the program must belong to the acting center (centerID claim).
	if actorCenterID == 0 || program.BeautyCenterID != actorCenterID {
		return response.LoyaltyProgramResponse{}, apperrors.ErrForbidden
	}

	if req.ServiceID != nil {
		program.ServiceID = req.ServiceID
	}
	if req.RewardServiceID != nil {
		program.RewardServiceID = req.RewardServiceID
	}
	if req.RequiredSteps != nil {
		program.RequiredSteps = *req.RequiredSteps
	}
	if req.RewardType != nil {
		program.RewardType = *req.RewardType
	}
	if req.DiscountValue != nil {
		program.DiscountValue = *req.DiscountValue
	}
	if req.DurationDays != nil {
		program.DurationDays = req.DurationDays
	}

	if err := s.repo.UpdateProgram(program); err != nil {
		return response.LoyaltyProgramResponse{}, err
	}

	return response.LoyaltyProgramResponse{
		ID:              program.ID,
		ServiceID:       program.ServiceID,
		RewardServiceID: program.RewardServiceID,
		RequiredSteps:   program.RequiredSteps,
		RewardType:      program.RewardType,
		DiscountValue:   program.DiscountValue,
		IsActive:        program.IsActive,
		CreatedAt:       program.CreatedAt,
	}, nil
}

func (s *LoyaltyServiceImpl) DeleteProgram(id, actorCenterID uint) error {
	program, err := s.repo.FindProgramByID(id)
	if err != nil {
		return err
	}
	if program == nil {
		return apperrors.ErrNotFound
	}
	if actorCenterID == 0 || program.BeautyCenterID != actorCenterID {
		return apperrors.ErrForbidden
	}
	return s.repo.DeleteProgram(program)
}

func (s *LoyaltyServiceImpl) ToggleProgram(id, actorCenterID uint, isActive bool) error {
	program, err := s.repo.FindProgramByID(id)
	if err != nil {
		return err
	}
	if program == nil {
		return apperrors.ErrNotFound
	}
	if actorCenterID == 0 || program.BeautyCenterID != actorCenterID {
		return apperrors.ErrForbidden
	}
	program.IsActive = isActive
	return s.repo.UpdateProgram(program)
}

func (s *LoyaltyServiceImpl) GetCenterSettings(beautyCenterID, actorCenterID uint) (response.LoyaltySettingsResponse, error) {
	// Reject a :centerId path value that does not match the caller's claim.
	if actorCenterID == 0 || actorCenterID != beautyCenterID {
		return response.LoyaltySettingsResponse{}, apperrors.ErrForbidden
	}

	// FindByID reports a missing/soft-deleted center as ErrRecordNotFound, so
	// both branches collapse to the same sentinel (no raw DB error leaks).
	center, err := s.centerRepo.FindByID(beautyCenterID)
	if err != nil || center == nil {
		return response.LoyaltySettingsResponse{}, apperrors.ErrNotFound
	}
	return response.LoyaltySettingsResponse{
		LoyaltyEnabled:      center.LoyaltyEnabled,
		LoyaltyPerPersonnel: center.LoyaltyPerPersonnel,
	}, nil
}

func (s *LoyaltyServiceImpl) UpdateCenterSettings(beautyCenterID, actorCenterID uint, req request.UpdateLoyaltySettingsRequest) error {
	// Reject a :centerId path value that does not match the caller's claim.
	if actorCenterID == 0 || actorCenterID != beautyCenterID {
		return apperrors.ErrForbidden
	}

	// FindByID reports a missing/soft-deleted center as ErrRecordNotFound, so
	// both branches collapse to the same sentinel (no raw DB error leaks).
	center, err := s.centerRepo.FindByID(beautyCenterID)
	if err != nil || center == nil {
		return apperrors.ErrNotFound
	}
	if req.LoyaltyEnabled != nil {
		center.LoyaltyEnabled = *req.LoyaltyEnabled
		// Sync all programs with the master toggle
		if err := s.repo.UpdateProgramsActiveByCenter(beautyCenterID, *req.LoyaltyEnabled); err != nil {
			return err
		}
	}
	if req.LoyaltyPerPersonnel != nil {
		center.LoyaltyPerPersonnel = *req.LoyaltyPerPersonnel
	}
	return s.centerRepo.Update(center)
}

// GetMetrics returns per-program issued-vs-redeemed counts for a center.
// Ownership: actorCenterID must equal beautyCenterID (403 otherwise), same
// contract as the other center-scoped loyalty reads.
func (s *LoyaltyServiceImpl) GetMetrics(beautyCenterID, actorCenterID uint) ([]response.LoyaltyMetricsResponse, error) {
	if actorCenterID == 0 || actorCenterID != beautyCenterID {
		return nil, apperrors.ErrForbidden
	}

	programs, err := s.repo.FindProgramsByCenter(beautyCenterID)
	if err != nil {
		return nil, err
	}

	res := make([]response.LoyaltyMetricsResponse, 0, len(programs))
	for _, p := range programs {
		active, completed, err := s.repo.CountCardsByProgramStatus(p.ID)
		if err != nil {
			return nil, err
		}
		redeemed, err := s.repo.CountRedemptionsByProgram(p.ID)
		if err != nil {
			return nil, err
		}
		issued := active + completed
		rate := 0.0
		if issued > 0 {
			rate = float64(redeemed) / float64(issued)
		}
		res = append(res, response.LoyaltyMetricsResponse{
			ProgramID:      p.ID,
			ServiceName:    p.Service.Name,
			RequiredSteps:  p.RequiredSteps,
			ActiveCards:    active,
			CompletedCards: completed,
			Redemptions:    redeemed,
			RedemptionRate: rate,
		})
	}
	return res, nil
}
