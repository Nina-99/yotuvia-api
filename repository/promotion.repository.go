package repository

import "github.com/Nina-99/yottu-api/models"

type PromotionRepository interface {
	Create(promotion *models.Promotion) error
	FindByID(id uint) (*models.Promotion, error)
	FindByCenter(beautyCenterID uint, includeInactive bool) ([]models.Promotion, error)
	FindActiveForService(beautyCenterID, serviceID uint, hour, dayOfWeek int) ([]models.Promotion, error)
	FindActiveForCenter(beautyCenterID uint, hour, dayOfWeek int) ([]models.Promotion, error)
	Update(promotion *models.Promotion) error
	Delete(promotion *models.Promotion) error
	CreateRedemption(redemption *models.PromotionRedemption) error
	IncrementRedemptionCount(promotionID uint) error
	FindRedemptionByReservation(reservationID uint) (*models.PromotionRedemption, error)
}
