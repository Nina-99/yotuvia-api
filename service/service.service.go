package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
)

type ServiceService interface {
	Create(req request.CreateServiceRequest) error
	Update(serviceID uint, req request.UpdateServiceRequest) error
	Delete(serviceID uint) error
	FindAll(page, limit int) ([]models.Service, int64, error)
	FindById(serviceID uint) (*models.Service, error)
}
