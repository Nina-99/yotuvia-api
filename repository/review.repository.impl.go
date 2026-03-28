package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type ReviewRepositoryImpl struct {
	db *gorm.DB
}

func NewReviewRepositoryImpl(db *gorm.DB) ReviewRepository {
	return &ReviewRepositoryImpl{db: db}
}

func (r *ReviewRepositoryImpl) Create(review *models.Review) error {
	return r.db.Create(review).Error
}

func (r *ReviewRepositoryImpl) Update(review *models.Review) error {
	return r.db.Save(review).Error
}

func (r *ReviewRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.Review{}, id).Error
}

func (r *ReviewRepositoryImpl) FindByID(id uint) (*models.Review, error) {
	var review models.Review
	err := r.db.Preload("Client").Preload("BeautyCenter").First(&review, id).Error
	if err != nil {
		return nil, err
	}
	return &review, nil
}

func (r *ReviewRepositoryImpl) FindByBeautyCenterID(beautyCenterID uint) ([]models.Review, error) {
	var reviews []models.Review
	err := r.db.Where("beauty_center_id = ?", beautyCenterID).Preload("Client").Preload("BeautyCenter").Find(&reviews).Error
	return reviews, err
}

func (r *ReviewRepositoryImpl) FindByClientID(clientID uint) ([]models.Review, error) {
	var reviews []models.Review
	err := r.db.Where("client_id = ?", clientID).Preload("Client").Preload("BeautyCenter").Find(&reviews).Error
	return reviews, err
}
