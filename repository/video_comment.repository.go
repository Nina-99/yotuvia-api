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
	// CountByFeedPostIDs counts top-level comments per post in ONE grouped
	// query, mirroring CountByFeedPostID's semantics (parent_id IS NULL,
	// soft-deleted rows excluded). Posts with no comments are absent from the
	// map rather than present as zero.
	CountByFeedPostIDs(ids []uint) (map[uint]int64, error)
}
