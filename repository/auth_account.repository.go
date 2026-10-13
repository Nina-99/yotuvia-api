package repository

import "github.com/Nina-99/yotuvia-api/models"

type AuthAccountRepository interface {
	Create(account *models.AuthAccount) error
	Update(account *models.AuthAccount) error
	Delete(id uint) error
	FindAll(page, limit int) ([]models.AuthAccount, int64, error)
	FindById(id uint) (*models.AuthAccount, error)
	FindByProvider(provider, providerId string) (*models.AuthAccount, error)

	// FindByLocalUser returns the local (password) AuthAccount linked to a
	// polymorphic (userType, userID), or (nil, nil) when none exists.
	FindByLocalUser(userType string, userID uint) (*models.AuthAccount, error)
}
