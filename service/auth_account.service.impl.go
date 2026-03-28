package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/go-playground/validator/v10"
)

type AuthAccountServiceImpl struct {
	AuthAccountRepository repository.AuthAccountRepository
	Validate              *validator.Validate
}

func NewAuthAccountServiceImpl(authAccountRepository repository.AuthAccountRepository, validate *validator.Validate) AuthAccountService {
	return &AuthAccountServiceImpl{
		AuthAccountRepository: authAccountRepository,
		Validate:              validate,
	}
}

func (s *AuthAccountServiceImpl) Create(req request.AuthAccountCreateRequest) (*response.AuthAccountResponse, error) {
	if err := s.Validate.Struct(req); err != nil {
		return nil, err
	}

	existing, err := s.AuthAccountRepository.FindByProvider(req.Provider, req.ProviderID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("auth account already exists for this provider and provider id")
	}

	account := models.AuthAccount{
		ClientID:   req.ClientID,
		Provider:   req.Provider,
		ProviderID: req.ProviderID,
	}

	if err := s.AuthAccountRepository.Create(&account); err != nil {
		return nil, err
	}

	// Re-fetch to get preloaded client
	created, err := s.AuthAccountRepository.FindById(account.ID)
	if err != nil {
		return nil, err
	}

	return s.toResponse(created), nil
}

func (s *AuthAccountServiceImpl) Update(id uint, req request.AuthAccountUpdateRequest) (*response.AuthAccountResponse, error) {
	if err := s.Validate.Struct(req); err != nil {
		return nil, err
	}

	account, err := s.AuthAccountRepository.FindById(id)
	if err != nil {
		return nil, err
	}

	if req.ProviderID != "" {
		account.ProviderID = req.ProviderID
	}

	if err := s.AuthAccountRepository.Update(account); err != nil {
		return nil, err
	}

	return s.toResponse(account), nil
}

func (s *AuthAccountServiceImpl) Delete(id uint) error {
	_, err := s.AuthAccountRepository.FindById(id)
	if err != nil {
		return err
	}
	return s.AuthAccountRepository.Delete(id)
}

func (s *AuthAccountServiceImpl) FindAll() ([]response.AuthAccountResponse, error) {
	accounts, err := s.AuthAccountRepository.FindAll()
	if err != nil {
		return nil, err
	}

	var res []response.AuthAccountResponse
	for _, acc := range accounts {
		res = append(res, *s.toResponse(&acc))
	}
	return res, nil
}

func (s *AuthAccountServiceImpl) FindById(id uint) (*response.AuthAccountResponse, error) {
	account, err := s.AuthAccountRepository.FindById(id)
	if err != nil {
		return nil, err
	}
	return s.toResponse(account), nil
}

func (s *AuthAccountServiceImpl) toResponse(account *models.AuthAccount) *response.AuthAccountResponse {
	res := &response.AuthAccountResponse{
		ID:         account.ID,
		ClientID:   account.ClientID,
		Provider:   account.Provider,
		ProviderID: account.ProviderID,
	}

	if account.Client.ID != 0 {
		res.Client = &response.ClientResponse{
			ID:        account.Client.ID,
			FirstName: account.Client.FirstName,
			LastName:  account.Client.LastName,
			Username:  account.Client.Username,
			Email:     account.Client.Email,
			Phone:     account.Client.Phone,
		}
	}

	return res
}
