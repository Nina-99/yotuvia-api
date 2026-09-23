package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
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
		ID:                program.ID,
		ServiceID:         program.ServiceID,
		RewardServiceID:   program.RewardServiceID,
		RequiredSteps:     program.RequiredSteps,
		RewardType:        program.RewardType,
		DiscountValue:     program.DiscountValue,
		IsActive:          program.IsActive,
		CreatedAt:         program.CreatedAt,
	}, nil
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
	// 3. If there is no active, we create one
	if card == nil {
		card = &models.CustomerPunchCard{
			ClientID:         clientID,
			LoyaltyProgramID: program.ID,
			CurrentSteps:     0,
			Status:           "active",
		}
		if err := s.repo.CreatePunchCard(card); err != nil {
			return err
		}
	}

	card.CurrentSteps++

	// 4. Audit the stamp history
	history := models.PunchHistory{
		CustomerPunchCardID: card.ID,
		ReservationID:       reservationID,
	}
	if err := s.repo.CreatePunchHistory(&history); err != nil {
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

func (s *LoyaltyServiceImpl) GetClientsCards(clientID uint) ([]response.PunchCardResponse, error) {
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
			LoyaltyProgram: response.LoyaltyProgramResponse{
				ID:            c.LoyaltyProgram.ID,
				ServiceID:     c.LoyaltyProgram.ServiceID,
				RequiredSteps: c.LoyaltyProgram.RequiredSteps,
				RewardType:    c.LoyaltyProgram.RewardType,
				DiscountValue: c.LoyaltyProgram.DiscountValue,
				IsActive:      c.LoyaltyProgram.IsActive,
				CreatedAt:     c.LoyaltyProgram.CreatedAt,
			},
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

	// Reset the card for next cycle
	cardPtr.CurrentSteps = 0
	cardPtr.Status = "active"
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

func (s *LoyaltyServiceImpl) FindProgramsByCenter(beautyCenterID uint) ([]response.LoyaltyProgramResponse, error) {
	programs, err := s.repo.FindProgramsByCenter(beautyCenterID)
	if err != nil {
		return nil, err
	}
	var res []response.LoyaltyProgramResponse
	for _, p := range programs {
		resp := response.LoyaltyProgramResponse{
			ID:                p.ID,
			ServiceID:         p.ServiceID,
			RewardServiceID:   p.RewardServiceID,
			RequiredSteps:     p.RequiredSteps,
			RewardType:        p.RewardType,
			DiscountValue:     p.DiscountValue,
			IsActive:          p.IsActive,
			CreatedAt:         p.CreatedAt,
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

func (s *LoyaltyServiceImpl) UpdateProgram(id uint, req request.UpdateLoyaltyProgramRequest) (response.LoyaltyProgramResponse, error) {
	program, err := s.repo.FindProgramByID(id)
	if err != nil || program == nil {
		return response.LoyaltyProgramResponse{}, errors.New("program not found")
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

	if err := s.repo.UpdateProgram(program); err != nil {
		return response.LoyaltyProgramResponse{}, err
	}

	return response.LoyaltyProgramResponse{
		ID:                program.ID,
		ServiceID:         program.ServiceID,
		RewardServiceID:   program.RewardServiceID,
		RequiredSteps:     program.RequiredSteps,
		RewardType:        program.RewardType,
		DiscountValue:     program.DiscountValue,
		IsActive:          program.IsActive,
		CreatedAt:         program.CreatedAt,
	}, nil
}

func (s *LoyaltyServiceImpl) DeleteProgram(id uint) error {
	program, err := s.repo.FindProgramByID(id)
	if err != nil || program == nil {
		return errors.New("program not found")
	}
	return s.repo.DeleteProgram(program)
}

func (s *LoyaltyServiceImpl) ToggleProgram(id uint, isActive bool) error {
	program, err := s.repo.FindProgramByID(id)
	if err != nil || program == nil {
		return errors.New("program not found")
	}
	program.IsActive = isActive
	return s.repo.UpdateProgram(program)
}

func (s *LoyaltyServiceImpl) GetCenterSettings(beautyCenterID uint) (response.LoyaltySettingsResponse, error) {
	center, err := s.centerRepo.FindByID(beautyCenterID)
	if err != nil || center == nil {
		return response.LoyaltySettingsResponse{}, errors.New("center not found")
	}
	return response.LoyaltySettingsResponse{
		LoyaltyEnabled:      center.LoyaltyEnabled,
		LoyaltyPerPersonnel: center.LoyaltyPerPersonnel,
	}, nil
}

func (s *LoyaltyServiceImpl) UpdateCenterSettings(beautyCenterID uint, req request.UpdateLoyaltySettingsRequest) error {
	center, err := s.centerRepo.FindByID(beautyCenterID)
	if err != nil || center == nil {
		return errors.New("center not found")
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
