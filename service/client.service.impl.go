package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type ClientServiceImpl struct {
	ClientRepository repository.ClientRepository
	Validate         *validator.Validate
}

func NewClientServiceImpl(clientRepository repository.ClientRepository, validate *validator.Validate) ClientService {
	return &ClientServiceImpl{
		ClientRepository: clientRepository,
		Validate:         validate,
	}
}

func (s *ClientServiceImpl) Update(id uint, actorClientID uint, req request.UpdateClientRequest) error {
	// Ownership at entry: a client may only update its own profile (:id == claim).
	if actorClientID != id {
		return apperrors.ErrForbidden
	}

	if err := s.Validate.Struct(req); err != nil {
		utils.Log.Warnf("Update failed: validation error: %v", err)
		return apperrors.ErrValidation
	}

	client, err := s.ClientRepository.FindById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrNotFound
		}
		utils.Log.Errorf("Update failed: client ID %d not found: %v", id, err)
		return err
	}

	if req.FirstName != "" {
		client.FirstName = req.FirstName
	}
	if req.LastName != "" {
		client.LastName = req.LastName
	}
	if req.Username != "" && req.Username != client.Username {
		existingUsername, err := s.ClientRepository.FindByUsername(req.Username)
		if err != nil {
			utils.Log.Errorf("Update failed: error checking username %s: %v", req.Username, err)
			return err
		}
		if existingUsername != nil {
			utils.Log.Warnf("Update failed: username %s already exists", req.Username)
			return apperrors.ErrConflict
		}
		client.Username = req.Username
	}
	if req.Email != "" && req.Email != client.Email {
		existingEmail, err := s.ClientRepository.FindByEmail(req.Email)
		if err != nil {
			utils.Log.Errorf("Update failed: error checking email %s: %v", req.Email, err)
			return err
		}
		if existingEmail != nil {
			utils.Log.Warnf("Update failed: email %s already exists", req.Email)
			return apperrors.ErrConflict
		}
		client.Email = req.Email
	}
	if req.Password != "" {
		hash, err := utils.HashPassword(req.Password)
		if err != nil {
			utils.Log.Errorf("Update failed: failed to hash password: %v", err)
			return err
		}
		client.Password = hash
	}
	client.IsActive = req.IsActive

	if err := s.ClientRepository.Update(client); err != nil {
		utils.Log.Errorf("Update failed: error saving client to database: %v", err)
		return err
	}
	utils.Log.Infof("Client with ID %d updated successfully", id)
	return nil
}

func (s *ClientServiceImpl) Delete(id uint, actorClientID uint) error {
	// Ownership at entry: a client may only delete its own profile.
	if actorClientID != id {
		return apperrors.ErrForbidden
	}

	user, err := s.ClientRepository.FindById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrNotFound
		}
		return err
	}
	return s.ClientRepository.Delete(user)
}

func (s *ClientServiceImpl) FindAll(page, limit int) ([]models.Client, int64, error) {
	return s.ClientRepository.FindAll(page, limit)
}

func (s *ClientServiceImpl) FindEmail(Email string) (*models.Client, error) {
	client, err := s.ClientRepository.FindByEmail(Email)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, apperrors.ErrNotFound
	}
	return client, nil
}

func (s *ClientServiceImpl) FindByID(id uint, actorClientID uint) (*models.Client, error) {
	// Ownership at entry: a client may only read its own profile.
	if actorClientID != id {
		return nil, apperrors.ErrForbidden
	}

	client, err := s.ClientRepository.FindById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return client, nil
}
