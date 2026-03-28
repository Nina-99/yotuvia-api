package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type AuthAccountService interface {
	Create(req request.AuthAccountCreateRequest) (*response.AuthAccountResponse, error)
	Update(id uint, req request.AuthAccountUpdateRequest) (*response.AuthAccountResponse, error)
	Delete(id uint) error
	FindAll() ([]response.AuthAccountResponse, error)
	FindById(id uint) (*response.AuthAccountResponse, error)
}
