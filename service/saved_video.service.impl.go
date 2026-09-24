package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type SavedVideoServiceImpl struct {
	SavedVideoRepository repository.SavedVideoRepository
	Validate             *validator.Validate
}

func NewSavedVideoServiceImpl(repo repository.SavedVideoRepository, validate *validator.Validate) SavedVideoService {
	return &SavedVideoServiceImpl{
		SavedVideoRepository: repo,
		Validate:             validate,
	}
}

func (s *SavedVideoServiceImpl) Toggle(clientID uint, postID uint) (bool, error) {
	// Delegate to repository atomic toggle (handles soft-delete correctly)
	return s.SavedVideoRepository.Toggle(postID, clientID)
}

func (s *SavedVideoServiceImpl) CheckIfSaved(clientID uint, postID uint) (bool, error) {
	_, err := s.SavedVideoRepository.FindByPostAndClient(postID, clientID)
	if err != nil {
		// errors.Is, not ==: the sentinel may come wrapped by the driver.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *SavedVideoServiceImpl) ListByClient(clientID uint) ([]response.SavedVideoResponse, error) {
	savedVideos, err := s.SavedVideoRepository.FindByClientID(clientID)
	if err != nil {
		return nil, err
	}

	var responses []response.SavedVideoResponse
	for _, sv := range savedVideos {
		responses = append(responses, response.SavedVideoResponse{
			ID:         sv.ID,
			FeedPostID: sv.FeedPostID,
			ClientID:   sv.ClientID,
			CreatedAt:  sv.CreatedAt,
		})
	}
	return responses, nil
}
