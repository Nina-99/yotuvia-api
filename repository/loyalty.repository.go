package repository

import "github.com/Nina-99/yottu-api/models"

type LoyaltyRepository interface {
	CreateProgram(program *models.LoyaltyProgram) error
	FindProgramByService(beautyCenterID, serviceID uint) (*models.LoyaltyProgram, error)
	FindActiveCard(clientID, programID uint) (*models.CustomerPunchCard, error)
	CreatePunchCard(card *models.CustomerPunchCard) error
	UpdatePunchCard(card *models.CustomerPunchCard) error
	CreatePunchHistory(history *models.PunchHistory) error
	FindCardsByClient(clientID uint) ([]models.CustomerPunchCard, error)
}
