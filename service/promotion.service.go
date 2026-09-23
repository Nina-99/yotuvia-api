package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type PromotionService interface {
	Create(req request.CreatePromotionRequest) (response.PromotionResponse, error)
	FindByID(id uint) (*response.PromotionResponse, error)
	FindByCenter(beautyCenterID uint, includeInactive bool) ([]response.PromotionResponse, error)
	Update(id uint, req request.UpdatePromotionRequest) (response.PromotionResponse, error)
	Delete(id uint) error
	// FindApplicable returns promotions that apply to the given service at the
	// current time (hour + dayOfWeek). Used by the reservation flow.
	FindApplicable(beautyCenterID, serviceID uint, hour, dayOfWeek int) ([]response.PromotionResponse, error)
	// CalculateDiscount computes the discount from a promotion. Returns the
	// discount amount to subtract from the reservation total.
	CalculateDiscount(promoID uint, servicePrice float64, serviceCount int) (float64, error)
	// FinalizeRedemption records that a promotion was applied to a reservation.
	FinalizeRedemption(promoID, reservationID uint, discount float64) error
}
