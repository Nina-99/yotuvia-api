package service

import (
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
)

type ServiceTemplate struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ServiceService interface {
	// Create/Update/Delete are centre-scoped: actorCenterID is the caller's
	// `center_id` claim. A foreign id is ErrForbidden and never reaches a write.
	Create(actorCenterID uint, req request.CreateServiceRequest) (uint, error)
	Update(actorCenterID uint, serviceID uint, req request.UpdateServiceRequest) error
	Delete(actorCenterID uint, serviceID uint) error
	FindAll(beautyCenterID uint, categoryID uint, isActive *bool, page, limit int) ([]models.Service, int64, error)
	FindById(serviceID uint) (*models.Service, error)
	GetServiceTemplates(categoryID uint, lang string) ([]ServiceTemplate, error)
}
