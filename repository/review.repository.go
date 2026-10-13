package repository

import "github.com/Nina-99/yotuvia-api/models"

type ReviewRepository interface {
	Create(review *models.Review) error
	Update(review *models.Review) error
	Delete(id uint) error
	FindByID(id uint) (*models.Review, error)
	FindByBeautyCenterID(beautyCenterID uint) ([]models.Review, error)
	FindByClientID(clientID uint) ([]models.Review, error)
	HasCompletedReservation(clientID, beautyCenterID uint) (bool, error)
}
