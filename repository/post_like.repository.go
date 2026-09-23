package repository

import "github.com/Nina-99/yottu-api/models"

type PostLikeRepository interface {
	Create(postLike *models.PostLike) error
	Delete(postLike *models.PostLike) error
	FindByPostAndClient(postID uint, clientID uint) (*models.PostLike, error)
	FindByPostID(postID uint) ([]models.PostLike, error)
	FindByClientID(clientID uint) ([]models.PostLike, error)
	// Toggle atomically creates or restores a like, returning the new liked state.
	Toggle(postID uint, clientID uint) (bool, error)
	// FindByPostAndClientUnscoped finds a like even if soft-deleted.
	FindByPostAndClientUnscoped(postID uint, clientID uint) (*models.PostLike, error)
}
