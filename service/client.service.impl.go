package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
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

// Login implements ClientService
func (s *ClientServiceImpl) Login(req request.LoginClientRequest) (string, error) {
	// Find username in database
	var client *models.Client
	var err error
	if utils.IsEmail(req.Identifier) {
		client, err = s.ClientRepository.FindByEmail(req.Identifier)
	} else {
		client, err = s.ClientRepository.FindByUsername(req.Identifier)
	}
	if err != nil {
		return "", errors.New("invalid credentials")
	}

	match := utils.VerifyPassword(req.Password, client.Password)
	if match == false {
		return "", errors.New("Invalid Password")
	}

	token, err := utils.GenerateJWT(client)
	if err != nil {
		return "", err
	}
	return token, nil
}

// Register implements ClientService
func (s *ClientServiceImpl) Register(req request.RegisterClientRequest) (string, error) {
	if err := s.Validate.Struct(req); err != nil {
		return "", err
	}

	existingEmail, err := s.ClientRepository.FindByEmail(req.Email)
	if err != nil {
		return "", err
	}
	if existingEmail != nil {
		return "", errors.New("email already exists")
	}

	existingUsername, err := s.ClientRepository.FindByUsername(req.Username)
	if err != nil {

		return "", err
	}
	if existingUsername != nil {
		return "", errors.New("username already exists")
	}
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return "", err
	}

	client := models.Client{FirstName: req.FirstName, LastName: req.LastName, Username: req.Username, Email: req.Email, Phone: req.Phone, Password: string(hash), IsActive: true}
	if err := s.ClientRepository.Create(&client); err != nil {
		return "", err
	}
	token, err := utils.GenerateJWT(&client)
	if err != nil {
		return "", err
	}
	return token, nil
}

// Update implements ClientServiceImpl
func (s *ClientServiceImpl) Update(clientID uint, req request.RegisterClientRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}

	client, err := s.ClientRepository.FindById(clientID)
	if err != nil {
		return err
	}

	if req.FirstName != "" {
		client.FirstName = req.FirstName
	}
	if req.LastName != "" {
		client.LastName = req.LastName
	}
	if req.Username != "" {
		client.Username = req.Username
	}
	if req.Email != "" {
		client.Email = req.Email
	}
	if req.Password != "" {
		hash, err := utils.HashPassword(req.Password)
		if err != nil {
			return err
		}
		client.Password = hash
	}
	client.IsActive = req.IsActive

	return s.ClientRepository.Update(client)
}

// Delete user implements ClientService
func (s *ClientServiceImpl) Delete(clientID uint) error {
	user, err := s.ClientRepository.FindById(clientID)
	if err != nil {
		return err
	}
	return s.ClientRepository.Delete(user)
}

func (s *ClientServiceImpl) FindAll() ([]models.Client, error) {
	return s.ClientRepository.FindAll()
}

func (s *ClientServiceImpl) FindEmail(Email string) (*models.Client, error) {
	return s.ClientRepository.FindByEmail(Email)
}
