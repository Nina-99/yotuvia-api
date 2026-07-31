package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
)

type ClientService interface {
	Update(clientID uint, req request.UpdateClientRequest) error
	Delete(clientID uint) error
	FindAll(page, limit int) ([]models.Client, int64, error)
	FindEmail(email string) (*models.Client, error)
	FindByID(clientID uint) (*models.Client, error)
}
