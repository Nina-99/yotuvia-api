package service

import (
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
)

type PromotionService interface {
	Create(req request.CreatePromotionRequest) (response.PromotionResponse, error)
	// FindByID/FindByCenter/Update/Delete are center-admin operations: the
	// actor's centerID claim is verified against row ownership and a mismatch
	// yields apperrors.ErrForbidden (403).
	FindByID(id, actorCenterID uint) (*response.PromotionResponse, error)
	FindByCenter(beautyCenterID, actorCenterID uint, includeInactive bool) ([]response.PromotionResponse, error)
	Update(id, actorCenterID uint, req request.UpdatePromotionRequest) (response.PromotionResponse, error)
	Delete(id, actorCenterID uint) error
	// FindApplicable returns promotions that apply to the given service at the
	// current time (hour + dayOfWeek). Used by the reservation flow.
	FindApplicable(beautyCenterID, serviceID uint, hour, dayOfWeek int) ([]response.PromotionResponse, error)
	// CalculateDiscount computes the discount from a promotion. Returns the
	// discount amount to subtract from the reservation total.
	CalculateDiscount(promoID uint, servicePrice float64, serviceCount int) (float64, error)
	// FinalizeRedemption records that a promotion was applied to a reservation.
	FinalizeRedemption(promoID, reservationID uint, discount float64) error
}
