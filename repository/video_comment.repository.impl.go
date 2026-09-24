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
	// Preload the direct replies so the list response carries them in the SAME
	// payload the client expects. GORM issues one extra SELECT for the whole
	// page (WHERE parent_id IN (...)), never one per parent. Replies are ordered
	// created_at ASC, matching FindReplies exactly. Hidden rows are NOT filtered
	// here: the response exposes is_hidden and the client decides, which is the
	// existing moderation contract (GetCommentReplies behaves the same way).
	query := r.db.
		Preload("Replies", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Where("feed_post_id = ? AND parent_id IS NULL", feedPostID).
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

// CountByFeedPostIDs runs one grouped COUNT for a page of posts. It preserves
// CountByFeedPostID's semantics exactly: top-level comments only (parent_id IS
// NULL) and soft-deleted rows excluded by GORM's default scope.
func (r *VideoCommentRepositoryImpl) CountByFeedPostIDs(ids []uint) (map[uint]int64, error) {
	counts := make(map[uint]int64, len(ids))
	if len(ids) == 0 {
		return counts, nil
	}

	var rows []struct {
		FeedPostID uint
		Count      int64
	}
	err := r.db.Model(&models.VideoComment{}).
		Select("feed_post_id, COUNT(*) AS count").
		Where("feed_post_id IN ? AND parent_id IS NULL", ids).
		Group("feed_post_id").
		Scan(&rows).Error
	if err != nil {
		return counts, err
	}
	for _, row := range rows {
		counts[row.FeedPostID] = row.Count
	}
	return counts, nil
}
