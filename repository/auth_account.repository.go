package repository

import "github.com/Nina-99/yottu-api/models"

type AuthAccountRepository interface {
	Create(account *models.AuthAccount) error
	Update(account *models.AuthAccount) error
	Delete(id uint) error
	FindAll() ([]models.AuthAccount, error)
	FindById(id uint) (*models.AuthAccount, error)
	FindByProvider(provider, providerId string) (*models.AuthAccount, error)
}
