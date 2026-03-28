package repository

import "github.com/Nina-99/yottu-api/models"

type PostLikeRepository interface {
	Create(postLike *models.PostLike) error
	Delete(postLike *models.PostLike) error
	FindByPostAndClient(postID uint, clientID uint) (*models.PostLike, error)
	FindByPostID(postID uint) ([]models.PostLike, error)
}
