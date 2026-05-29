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
	repo     repository.LoyaltyRepository
	Validate *validator.Validate
}

func NewLoyaltyService(repo repository.LoyaltyRepository, validate *validator.Validate) LoyaltyService {
	return &LoyaltyServiceImpl{
		repo:     repo,
		Validate: validate,
	}
}

func (s *LoyaltyServiceImpl) CreateProgram(req request.CreateLoyaltyProgrmaRequest) (response.LoyaltyProgramResponse, error) {
	if err := s.Validate.Struct(req); err != nil {
		return response.LoyaltyProgramResponse{}, err
	}

	// Check if there is already an active promotion for this service at this beuty center
	existing, err := s.repo.FindProgramByService(req.BeautyCenterID, req.ServiceID)
	if err != nil {
		return response.LoyaltyProgramResponse{}, err
	}
	if existing != nil {
		return response.LoyaltyProgramResponse{}, errors.New("A loyalty program exists for this service")
	}

	program := models.LoyaltyProgram{
		BeautyCenterID: req.BeautyCenterID,
		ServiceID:      req.ServiceID,
		RequiredSteps:  req.RequiredSteps,
		RewardType:     req.RewardType,
		DiscountValue:  req.DiscountValue,
		IsActive:       true,
	}

	utils.Log.Info("[Loyalty] New program created for BeautyCenteer %d on Service %d", req.BeautyCenterID, req.ServiceID)

	return response.LoyaltyProgramResponse{
		ID:            program.ID,
		ServiceID:     program.ServiceID,
		RequiredSteps: program.RequiredSteps,
		RewardType:    program.RewardType,
		DiscountValue: program.DiscountValue,
		IsActive:      program.IsActive,
		CreatedAt:     program.CreatedAt,
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
