package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
)

type ClientService interface {
	Login(req request.LoginClientRequest) (string, error)
	Register(req request.RegisterClientRequest) (string, error)
	Update(clientID uint, req request.RegisterClientRequest) error
	Delete(clientID uint) error
	FindAll() ([]models.Client, error)
	FindEmail(email string) (*models.Client, error)
}
