package repository

import "github.com/Nina-99/yottu-api/models"

type FavoriteRepository interface {
	Create(favorite *models.Favorite) error
	Delete(id uint) error
	FindByClientID(clientID uint) ([]models.Favorite, error)
	IsFavorite(clientID uint, beautyCenterID uint) (bool, error)
}
