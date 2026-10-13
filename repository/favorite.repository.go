package repository

import "github.com/Nina-99/yotuvia-api/models"

type FavoriteRepository interface {
	Create(favorite *models.Favorite) error
	// FindByID returns the favorite or (nil, nil) when no row exists, so the
	// service can map a missing row to apperrors.ErrNotFound instead of letting
	// a no-op delete answer 200.
	FindByID(id uint) (*models.Favorite, error)
	Delete(id uint) error
	FindByClientID(clientID uint) ([]models.Favorite, error)
	IsFavorite(clientID uint, beautyCenterID uint) (bool, error)
}
