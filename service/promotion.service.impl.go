package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
)

type PromotionServiceImpl struct {
	repo     repository.PromotionRepository
	validate *validator.Validate
}

func NewPromotionService(repo repository.PromotionRepository, validate *validator.Validate) PromotionService {
	return &PromotionServiceImpl{repo: repo, validate: validate}
}

func (s *PromotionServiceImpl) toResponse(p *models.Promotion) response.PromotionResponse {
	resp := response.PromotionResponse{
		ID:                 p.ID,
		BeautyCenterID:     p.BeautyCenterID,
		Name:               p.Name,
		Description:        p.Description,
		Type:               p.Type,
		DiscountValue:      p.DiscountValue,
		BuyXCount:          p.BuyXCount,
		BuyYCount:          p.BuyYCount,
		ServiceID:          p.ServiceID,
		ValidFrom:          p.ValidFrom,
		ValidTo:            p.ValidTo,
		ApplicableHours:    p.ApplicableHours,
		TargetClients:      p.TargetClients,
		MaxRedemptions:     p.MaxRedemptions,
		CurrentRedemptions: p.CurrentRedemptions,
		IsActive:           p.IsActive,
		CreatedAt:          p.CreatedAt,
	}
	if p.Service.Name != "" {
		resp.ServiceName = p.Service.Name
	}
	return resp
}

func (s *PromotionServiceImpl) Create(req request.CreatePromotionRequest) (response.PromotionResponse, error) {
	if err := s.validate.Struct(req); err != nil {
		return response.PromotionResponse{}, err
	}

	if req.TargetClients == "" {
		req.TargetClients = "all"
	}
	if req.BuyXCount == 0 {
		req.BuyXCount = 1
	}
	if req.BuyYCount == 0 {
		req.BuyYCount = 1
	}

	promo := models.Promotion{
		BeautyCenterID:  req.BeautyCenterID,
		Name:            req.Name,
		Description:     req.Description,
		Type:            req.Type,
		DiscountValue:   req.DiscountValue,
		BuyXCount:       req.BuyXCount,
		BuyYCount:       req.BuyYCount,
		ServiceID:       req.ServiceID,
		ValidFrom:       req.ValidFrom,
		ValidTo:         req.ValidTo,
		ApplicableHours: req.ApplicableHours,
		TargetClients:   req.TargetClients,
		MaxRedemptions:  req.MaxRedemptions,
		IsActive:        true,
	}

	if err := s.repo.Create(&promo); err != nil {
		return response.PromotionResponse{}, err
	}

	utils.Log.Infof("[PROMO] Created promotion %d (%s) for center %d", promo.ID, promo.Name, promo.BeautyCenterID)
	return s.toResponse(&promo), nil
}

func (s *PromotionServiceImpl) FindByID(id uint) (*response.PromotionResponse, error) {
	promo, err := s.repo.FindByID(id)
	if err != nil || promo == nil {
		return nil, errors.New("promotion not found")
	}
	resp := s.toResponse(promo)
	return &resp, nil
}

func (s *PromotionServiceImpl) FindByCenter(beautyCenterID uint, includeInactive bool) ([]response.PromotionResponse, error) {
	promos, err := s.repo.FindByCenter(beautyCenterID, includeInactive)
	if err != nil {
		return nil, err
	}
	var res []response.PromotionResponse
	for _, p := range promos {
		res = append(res, s.toResponse(&p))
	}
	return res, nil
}

func (s *PromotionServiceImpl) Update(id uint, req request.UpdatePromotionRequest) (response.PromotionResponse, error) {
	promo, err := s.repo.FindByID(id)
	if err != nil || promo == nil {
		return response.PromotionResponse{}, errors.New("promotion not found")
	}

	if req.Name != nil {
		promo.Name = *req.Name
	}
	if req.Description != nil {
		promo.Description = *req.Description
	}
	if req.Type != nil {
		promo.Type = *req.Type
	}
	if req.DiscountValue != nil {
		promo.DiscountValue = *req.DiscountValue
	}
	if req.BuyXCount != nil {
		promo.BuyXCount = *req.BuyXCount
	}
	if req.BuyYCount != nil {
		promo.BuyYCount = *req.BuyYCount
	}
	if req.ServiceID != nil {
		promo.ServiceID = req.ServiceID
	}
	if req.ValidFrom != nil {
		promo.ValidFrom = req.ValidFrom
	}
	if req.ValidTo != nil {
		promo.ValidTo = req.ValidTo
	}
	if req.ApplicableHours != nil {
		promo.ApplicableHours = *req.ApplicableHours
	}
	if req.TargetClients != nil {
		promo.TargetClients = *req.TargetClients
	}
	if req.MaxRedemptions != nil {
		promo.MaxRedemptions = *req.MaxRedemptions
	}
	if req.IsActive != nil {
		promo.IsActive = *req.IsActive
	}

	if err := s.repo.Update(promo); err != nil {
		return response.PromotionResponse{}, err
	}
	return s.toResponse(promo), nil
}

func (s *PromotionServiceImpl) Delete(id uint) error {
	promo, err := s.repo.FindByID(id)
	if err != nil || promo == nil {
		return errors.New("promotion not found")
	}
	return s.repo.Delete(promo)
}

func (s *PromotionServiceImpl) FindApplicable(beautyCenterID, serviceID uint, hour, dayOfWeek int) ([]response.PromotionResponse, error) {
	promos, err := s.repo.FindActiveForService(beautyCenterID, serviceID, hour, dayOfWeek)
	if err != nil {
		return nil, err
	}

	var applicable []response.PromotionResponse
	for _, p := range promos {
		if s.isApplicableNow(&p, hour) {
			applicable = append(applicable, s.toResponse(&p))
		}
	}
	return applicable, nil
}

// isApplicableNow checks if the promotion's ApplicableHours include the given hour.
func (s *PromotionServiceImpl) isApplicableNow(p *models.Promotion, hour int) bool {
	if p.ApplicableHours == "" || p.ApplicableHours == "[]" {
		return true // no hour restriction
	}

	type TimeRange struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	var ranges []TimeRange
	if err := json.Unmarshal([]byte(p.ApplicableHours), &ranges); err != nil {
		return true // malformed → treat as always applicable
	}

	for _, r := range ranges {
		startHour := parseHour(r.Start)
		endHour := parseHour(r.End)
		if hour >= startHour && hour < endHour {
			return true
		}
	}
	return false
}

func parseHour(s string) int {
	var h, m int
	fmt.Sscanf(s, "%d:%d", &h, &m)
	return h
}

func (s *PromotionServiceImpl) CalculateDiscount(promoID uint, servicePrice float64, serviceCount int) (float64, error) {
	promo, err := s.repo.FindByID(promoID)
	if err != nil || promo == nil {
		return 0, errors.New("promotion not found")
	}

	now := time.Now()
	if promo.ValidFrom != nil && now.Before(*promo.ValidFrom) {
		return 0, nil
	}
	if promo.ValidTo != nil && now.After(*promo.ValidTo) {
		return 0, nil
	}
	if promo.MaxRedemptions > 0 && promo.CurrentRedemptions >= promo.MaxRedemptions {
		return 0, nil
	}

	switch promo.Type {
	case "percentage":
		return servicePrice * promo.DiscountValue / 100, nil
	case "fixed_amount":
		if promo.DiscountValue > servicePrice {
			return servicePrice, nil
		}
		return promo.DiscountValue, nil
	case "buy_x_get_y":
		if serviceCount < promo.BuyXCount {
			return 0, nil
		}
		freeCount := (serviceCount / promo.BuyXCount) * promo.BuyYCount
		return servicePrice * float64(freeCount), nil
	default:
		return 0, nil
	}
}

func (s *PromotionServiceImpl) FinalizeRedemption(promoID, reservationID uint, discount float64) error {
	redemption := models.PromotionRedemption{
		PromotionID:     promoID,
		ReservationID:   reservationID,
		DiscountApplied: discount,
	}
	if err := s.repo.CreateRedemption(&redemption); err != nil {
		return err
	}
	if err := s.repo.IncrementRedemptionCount(promoID); err != nil {
		return err
	}
	utils.Log.Infof("[PROMO] Redemption recorded: promotion %d applied to reservation %d, discount %.2f", promoID, reservationID, discount)
	return nil
}
