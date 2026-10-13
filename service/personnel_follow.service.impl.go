package service

import (
	"errors"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type PersonnelFollowServiceImpl struct {
	FollowRepository    repository.PersonnelFollowRepository
	PersonnelRepository repository.PersonnelRepository
	Validate            *validator.Validate
}

func NewPersonnelFollowServiceImpl(
	followRepo repository.PersonnelFollowRepository,
	personnelRepo repository.PersonnelRepository,
	validate *validator.Validate,
) PersonnelFollowService {
	return &PersonnelFollowServiceImpl{
		FollowRepository:    followRepo,
		PersonnelRepository: personnelRepo,
		Validate:            validate,
	}
}

// requireActivePersonnel resolves the target professional and rejects a
// missing or inactive one with ErrNotFound (the project's canonical error for
// an unavailable resource).
func (s *PersonnelFollowServiceImpl) requireActivePersonnel(personnelID uint) error {
	personnel, err := s.PersonnelRepository.FindByID(personnelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrNotFound
		}
		return err
	}
	if personnel == nil || !personnel.IsActive {
		return apperrors.ErrNotFound
	}
	return nil
}

// Follow implements PersonnelFollowService. Idempotent: following twice keeps
// a single row and returns nil both times.
func (s *PersonnelFollowServiceImpl) Follow(clientID, personnelID uint) error {
	if err := s.requireActivePersonnel(personnelID); err != nil {
		return err
	}
	return s.FollowRepository.Follow(&models.PersonnelFollow{
		ClientID:    clientID,
		PersonnelID: personnelID,
	})
}

// Unfollow implements PersonnelFollowService. Idempotent: removing a missing
// edge is a no-op that still returns nil.
func (s *PersonnelFollowServiceImpl) Unfollow(clientID, personnelID uint) error {
	if err := s.requireActivePersonnel(personnelID); err != nil {
		return err
	}
	return s.FollowRepository.Unfollow(clientID, personnelID)
}

// IsFollowing implements PersonnelFollowService.
func (s *PersonnelFollowServiceImpl) IsFollowing(clientID, personnelID uint) (bool, error) {
	return s.FollowRepository.IsFollowing(clientID, personnelID)
}
