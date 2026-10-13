package service

import (
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
)

type PostLikeService interface {
	Create(clientID uint, postID uint) (*models.PostLike, error)
	Delete(clientID uint, postID uint) error
	GetLikesByPostID(postID uint) ([]response.PostLikeResponse, error)
	CheckIfUserLiked(clientID uint, postID uint) (bool, error)
	// Toggle atomically creates or removes a like, returns new liked state.
	Toggle(clientID uint, postID uint) (bool, error)
}
