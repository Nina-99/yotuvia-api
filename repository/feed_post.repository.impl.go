package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type FeedPostRepositoryImpl struct {
	db *gorm.DB
}

func NewFeedPostRepositoryImpl(db *gorm.DB) FeedPostRepository {
	return &FeedPostRepositoryImpl{db: db}
}

// Create implements FeedPostRepository
func (r *FeedPostRepositoryImpl) Create(post *models.FeedPost) error {
	return r.db.Create(post).Error
}

// Update implements FeedPostRepository
func (r *FeedPostRepositoryImpl) Update(post *models.FeedPost) error {
	return r.db.Save(post).Error
}

// Delete implements FeedPostRepository
func (r *FeedPostRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.FeedPost{}, id).Error
}

// FindByID implements FeedPostRepository
func (r *FeedPostRepositoryImpl) FindByID(id uint) (*models.FeedPost, error) {
	var post models.FeedPost
	err := r.db.Preload("BeautyCenter").Preload("Likes").Preload("Service").Preload("Personnel").First(&post, id).Error
	if err != nil {
		return nil, err
	}
	return &post, nil
}

// FindAll implements FeedPostRepository
func (r *FeedPostRepositoryImpl) FindAll() ([]models.FeedPost, error) {
	var posts []models.FeedPost
	err := r.db.Preload("BeautyCenter").Preload("Likes").Preload("Service").Preload("Personnel").Order("created_at desc").Find(&posts).Error
	return posts, err
}

// FindByBeautyCenterID implements FeedPostRepository
func (r *FeedPostRepositoryImpl) FindByBeautyCenterID(centerID uint) ([]models.FeedPost, error) {
	var posts []models.FeedPost
	err := r.db.Preload("BeautyCenter").Preload("Likes").Preload("Service").Preload("Personnel").Where("beauty_center_id = ?", centerID).Order("created_at desc").Find(&posts).Error
	return posts, err
}
