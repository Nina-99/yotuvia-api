package service

import (
	"errors"

	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type PostLikeServiceImpl struct {
	PostLikeRepository repository.PostLikeRepository
	Validate           *validator.Validate
}

func NewPostLikeServiceImpl(postLikeRepository repository.PostLikeRepository, validate *validator.Validate) PostLikeService {
	return &PostLikeServiceImpl{
		PostLikeRepository: postLikeRepository,
		Validate:           validate,
	}
}

func (s *PostLikeServiceImpl) Create(clientID uint, postID uint) (*models.PostLike, error) {
	// Check if already liked
	existing, err := s.PostLikeRepository.FindByPostAndClient(postID, clientID)
	if err == nil && existing != nil {
		return nil, errors.New("already liked this post")
	}

	postLike := models.PostLike{
		FeedPostID: postID,
		ClientID:   clientID,
	}

	err = s.PostLikeRepository.Create(&postLike)
	if err != nil {
		return nil, err
	}

	return &postLike, nil
}

func (s *PostLikeServiceImpl) Delete(clientID uint, postID uint) error {
	postLike, err := s.PostLikeRepository.FindByPostAndClient(postID, clientID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("like not found")
		}
		return err
	}

	return s.PostLikeRepository.Delete(postLike)
}

func (s *PostLikeServiceImpl) GetLikesByPostID(postID uint) ([]response.PostLikeResponse, error) {
	likes, err := s.PostLikeRepository.FindByPostID(postID)
	if err != nil {
		return nil, err
	}

	var postLikeResponses []response.PostLikeResponse
	for _, like := range likes {
		postLikeResponses = append(postLikeResponses, response.PostLikeResponse{
			ID:         like.ID,
			FeedPostID: like.FeedPostID,
			ClientID:   like.ClientID,
			CreatedAt:  like.CreatedAt,
		})
	}

	return postLikeResponses, nil
}

func (s *PostLikeServiceImpl) CheckIfUserLiked(clientID uint, postID uint) (bool, error) {
	_, err := s.PostLikeRepository.FindByPostAndClient(postID, clientID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Toggle atomically creates or removes a like, returns new liked state.
func (s *PostLikeServiceImpl) Toggle(clientID uint, postID uint) (bool, error) {
	return s.PostLikeRepository.Toggle(postID, clientID)
}
