package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
)

type ServiceTemplate struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ServiceService interface {
	Create(req request.CreateServiceRequest) (uint, error)
	Update(serviceID uint, req request.UpdateServiceRequest) error
	Delete(serviceID uint) error
	FindAll(beautyCenterID uint, categoryID uint, isActive *bool, page, limit int) ([]models.Service, int64, error)
	FindById(serviceID uint) (*models.Service, error)
	GetServiceTemplates(categoryID uint, lang string) ([]ServiceTemplate, error)
}
