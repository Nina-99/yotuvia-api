package repository

import "github.com/Nina-99/yottu-api/models"

type ServiceRepository interface {
	Create(service *models.Service) error
	Update(service *models.Service) error
	Delete(service *models.Service) error
	FindById(serviceID uint) (*models.Service, error)
	FindAll(page, limit int) ([]models.Service, int64, error)
}
