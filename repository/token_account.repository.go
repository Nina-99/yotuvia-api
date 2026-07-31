package repository

import "github.com/Nina-99/yottu-api/models"

type TokenAccountRepository interface {
	Create(account *models.TokenAccount) error
	Update(account *models.TokenAccount) error
	FindByBeautyCenterID(beautyCenterID uint) (*models.TokenAccount, error)
}
