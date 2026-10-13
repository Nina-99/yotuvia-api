package repository

import (
	"errors"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type FavoriteRepositoryImpl struct {
	Db *gorm.DB
}

func NewFavoriteRepositoryImpl(db *gorm.DB) FavoriteRepository {
	return &FavoriteRepositoryImpl{Db: db}
}

func (r *FavoriteRepositoryImpl) Create(favorite *models.Favorite) error {
	return r.Db.Create(favorite).Error
}

// FindByID returns a single favorite by ID, or (nil, nil) when it does not
// exist (including soft-deleted rows), mirroring the other FindByID repositories
// so the service can distinguish "missing" (404) from a real DB failure.
func (r *FavoriteRepositoryImpl) FindByID(id uint) (*models.Favorite, error) {
	var favorite models.Favorite
	err := r.Db.First(&favorite, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &favorite, nil
}

func (r *FavoriteRepositoryImpl) Delete(id uint) error {
	return r.Db.Delete(&models.Favorite{}, id).Error
}

func (r *FavoriteRepositoryImpl) FindByClientID(clientID uint) ([]models.Favorite, error) {
	var favorites []models.Favorite
	err := r.Db.Preload("BeautyCenter").Where("client_id = ?", clientID).Find(&favorites).Error
	return favorites, err
}

func (r *FavoriteRepositoryImpl) IsFavorite(clientID uint, beautyCenterID uint) (bool, error) {
	var count int64
	err := r.Db.Model(&models.Favorite{}).Where("client_id = ? AND beauty_center_id = ?", clientID, beautyCenterID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
