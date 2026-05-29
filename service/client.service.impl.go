package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
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

// Update implements ClientServiceImpl
func (s *ClientServiceImpl) Update(clientID uint, req request.UpdateClientRequest) error {
	utils.Log.Infof("Updating client with ID: %d", clientID)
	if err := s.Validate.Struct(req); err != nil {
		utils.Log.Warnf("Update failed: validation error: %v", err)
		return err
	}

	client, err := s.ClientRepository.FindById(clientID)
	if err != nil {
		utils.Log.Errorf("Update failed: client ID %d not found: %v", clientID, err)
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
			return errors.New("username already exists")
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
			return errors.New("email already exists")
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
	utils.Log.Infof("Client with ID %d updated successfully", clientID)
	return nil
}

// Delete user implements ClientService
func (s *ClientServiceImpl) Delete(clientID uint) error {
	user, err := s.ClientRepository.FindById(clientID)
	if err != nil {
		return err
	}
	return s.ClientRepository.Delete(user)
}

// FindAll user implements ClientService
func (s *ClientServiceImpl) FindAll() ([]models.Client, error) {
	return s.ClientRepository.FindAll()
}

// FindEmail user implements ClientService
func (s *ClientServiceImpl) FindEmail(Email string) (*models.Client, error) {
	client, err := s.ClientRepository.FindByEmail(Email)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, errors.New("Client not found")
	}

	return client, nil
}
