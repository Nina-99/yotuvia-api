package repository

import (
	"errors"

	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type VideoRepositoryImpl struct {
	db *gorm.DB
}

func NewVideoRepositoryImpl(db *gorm.DB) VideoRepository {
	return &VideoRepositoryImpl{db: db}
}

func (r *VideoRepositoryImpl) Create(video *models.Video) error {
	return r.db.Create(video).Error
}

func (r *VideoRepositoryImpl) Delete(video *models.Video) error {
	return r.db.Delete(video).Error
}

func (r *VideoRepositoryImpl) FindByID(id uint) (*models.Video, error) {
	var video models.Video
	err := r.db.Preload("BeautyCenter").First(&video, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("video not found")
		}
		return nil, err
	}
	return &video, nil
}

func (r *VideoRepositoryImpl) FindAll(page, limit int) ([]models.Video, int64, error) {
	var videos []models.Video
	var total int64
	r.db.Model(&models.Video{}).Count(&total)
	offset := (page - 1) * limit
	err := r.db.
		Preload("BeautyCenter").
		Where("is_active = ?", true).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&videos).Error
	return videos, total, err
}

func (r *VideoRepositoryImpl) FindByBeautyCenterID(centerID uint, page, limit int) ([]models.Video, int64, error) {
	var videos []models.Video
	var total int64
	r.db.Model(&models.Video{}).Where("beauty_center_id = ?", centerID).Count(&total)
	offset := (page - 1) * limit
	err := r.db.
		Where("beauty_center_id = ? AND is_active = ?", centerID, true).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&videos).Error
	return videos, total, err
}

func (r *VideoRepositoryImpl) IncrementViews(id uint) error {
	return r.db.Model(&models.Video{}).Where("id = ?", id).
		UpdateColumn("views", gorm.Expr("views + 1")).Error
}
