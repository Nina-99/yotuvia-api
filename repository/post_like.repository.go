package repository

import "github.com/Nina-99/yottu-api/models"

type PostLikeRepository interface {
	Create(postLike *models.PostLike) error
	Delete(postLike *models.PostLike) error
	FindByPostAndClient(postID uint, clientID uint) (*models.PostLike, error)
	FindByPostID(postID uint) ([]models.PostLike, error)
	FindByClientID(clientID uint) ([]models.PostLike, error)
	// CountByFeedPostIDs counts live (non soft-deleted) likes per post in ONE
	// grouped query, for the feed's likes_count. Posts with no likes are absent
	// from the map rather than present as zero.
	CountByFeedPostIDs(ids []uint) (map[uint]int64, error)
	// Toggle atomically creates or restores a like, returning the new liked state.
	Toggle(postID uint, clientID uint) (bool, error)
	// FindByPostAndClientUnscoped finds a like even if soft-deleted.
	FindByPostAndClientUnscoped(postID uint, clientID uint) (*models.PostLike, error)
}
