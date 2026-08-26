package repository

import "github.com/Nina-99/yottu-api/models"

type VideoCommentRepository interface {
	Create(comment *models.VideoComment) error
	FindByID(id uint) (*models.VideoComment, error)
	FindByFeedPost(feedPostID uint, cursor *uint, limit int) ([]models.VideoComment, error)
	FindReplies(parentID uint) ([]models.VideoComment, error)
	Update(comment *models.VideoComment) error
	Delete(id uint) error
	CountByFeedPostID(feedPostID uint) (int64, error)
}
