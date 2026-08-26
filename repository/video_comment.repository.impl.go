package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type VideoCommentRepositoryImpl struct {
	db *gorm.DB
}

func NewVideoCommentRepositoryImpl(db *gorm.DB) VideoCommentRepository {
	return &VideoCommentRepositoryImpl{db: db}
}

func (r *VideoCommentRepositoryImpl) Create(comment *models.VideoComment) error {
	return r.db.Create(comment).Error
}

func (r *VideoCommentRepositoryImpl) FindByID(id uint) (*models.VideoComment, error) {
	var comment models.VideoComment
	err := r.db.Preload("Replies").First(&comment, id).Error
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *VideoCommentRepositoryImpl) FindByFeedPost(feedPostID uint, cursor *uint, limit int) ([]models.VideoComment, error) {
	var comments []models.VideoComment
	query := r.db.Where("feed_post_id = ? AND parent_id IS NULL", feedPostID).
		Order("created_at DESC")

	if cursor != nil {
		query = query.Where("id < ?", *cursor)
	}

	err := query.Limit(limit).Find(&comments).Error
	return comments, err
}

func (r *VideoCommentRepositoryImpl) FindReplies(parentID uint) ([]models.VideoComment, error) {
	var replies []models.VideoComment
	err := r.db.Where("parent_id = ?", parentID).
		Order("created_at ASC").
		Find(&replies).Error
	return replies, err
}

func (r *VideoCommentRepositoryImpl) Update(comment *models.VideoComment) error {
	return r.db.Save(comment).Error
}

func (r *VideoCommentRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.VideoComment{}, id).Error
}

func (r *VideoCommentRepositoryImpl) CountByFeedPostID(feedPostID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.VideoComment{}).
		Where("feed_post_id = ? AND parent_id IS NULL", feedPostID).
		Count(&count).Error
	return count, err
}
