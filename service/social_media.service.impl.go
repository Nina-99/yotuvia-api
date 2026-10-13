package service

import (
	"errors"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type SocialMediaServiceImpl struct {
	SocialMediaRepository repository.SocialMediaRepository
	Validate              *validator.Validate
}

func NewSocialMediaServiceImpl(repo repository.SocialMediaRepository, validate *validator.Validate) SocialMediaService {
	return &SocialMediaServiceImpl{
		SocialMediaRepository: repo,
		Validate:              validate,
	}
}

func (s *SocialMediaServiceImpl) Create(req request.CreateSocialMediaRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}

	// Check if already exists for this BeautyCenter
	existing, err := s.SocialMediaRepository.FindByBeautyCenterID(req.BeautyCenterID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if existing != nil {
		return errors.New("social media links already exist for this beauty center")
	}

	socialMedia := models.SocialMedia{
		BeautyCenterID: req.BeautyCenterID,
		Google:         req.Google,
		Facebook:       req.Facebook,
		Instagram:      req.Instagram,
		Twitter:        req.Twitter,
		TikTok:         req.TikTok,
	}

	return s.SocialMediaRepository.Create(&socialMedia)
}

func (s *SocialMediaServiceImpl) Update(id, actorCenterID uint, req request.UpdateSocialMediaRequest) error {
	socialMedia, err := s.SocialMediaRepository.FindByID(id)
	if err != nil {
		return apperrors.ErrNotFound
	}
	// Ownership first so a foreign admin always gets 403, never a validation
	// hint about a row it does not own.
	if actorCenterID == 0 || socialMedia.BeautyCenterID != actorCenterID {
		return apperrors.ErrForbidden
	}

	if err := s.Validate.Struct(req); err != nil {
		return err
	}

	socialMedia.Google = req.Google
	socialMedia.Facebook = req.Facebook
	socialMedia.Instagram = req.Instagram
	socialMedia.Twitter = req.Twitter
	socialMedia.TikTok = req.TikTok

	return s.SocialMediaRepository.Update(socialMedia)
}

func (s *SocialMediaServiceImpl) Delete(id, actorCenterID uint) error {
	socialMedia, err := s.SocialMediaRepository.FindByID(id)
	if err != nil {
		return apperrors.ErrNotFound
	}
	// Ownership: only the owning centre may delete its links.
	if actorCenterID == 0 || socialMedia.BeautyCenterID != actorCenterID {
		return apperrors.ErrForbidden
	}
	return s.SocialMediaRepository.Delete(socialMedia)
}

func (s *SocialMediaServiceImpl) FindByID(id uint) (*response.SocialMediaResponse, error) {
	socialMedia, err := s.SocialMediaRepository.FindByID(id)
	if err != nil {
		return nil, err
	}

	return &response.SocialMediaResponse{
		ID:             socialMedia.ID,
		BeautyCenterID: socialMedia.BeautyCenterID,
		Google:         socialMedia.Google,
		Facebook:       socialMedia.Facebook,
		Instagram:      socialMedia.Instagram,
		Twitter:        socialMedia.Twitter,
		TikTok:         socialMedia.TikTok,
		CreatedAt:      socialMedia.CreatedAt,
		UpdatedAt:      socialMedia.UpdatedAt,
	}, nil
}

func (s *SocialMediaServiceImpl) FindByBeautyCenterID(beautyCenterID uint) (*response.SocialMediaResponse, error) {
	socialMedia, err := s.SocialMediaRepository.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return nil, err
	}
	if socialMedia == nil {
		return nil, errors.New("social media links not found for this beauty center")
	}

	return &response.SocialMediaResponse{
		ID:             socialMedia.ID,
		BeautyCenterID: socialMedia.BeautyCenterID,
		Google:         socialMedia.Google,
		Facebook:       socialMedia.Facebook,
		Instagram:      socialMedia.Instagram,
		Twitter:        socialMedia.Twitter,
		TikTok:         socialMedia.TikTok,
		CreatedAt:      socialMedia.CreatedAt,
		UpdatedAt:      socialMedia.UpdatedAt,
	}, nil
}
