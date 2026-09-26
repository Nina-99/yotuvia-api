package repository

import "github.com/Nina-99/yottu-api/models"

type ServiceRepository interface {
	Create(service *models.Service) error
	Update(service *models.Service) error
	Delete(service *models.Service) error
	FindById(serviceID uint) (*models.Service, error)
	FindByIDs(ids []uint) ([]models.Service, error)
	// FindActiveByTreatmentIDs loads, in ONE query, every active service of the
	// candidate centers whose TreatmentID is in the requested set. It is the
	// batched backbone of the treatment-driven availability search: the caller
	// derives per-center durations from the result instead of querying per center.
	FindActiveByTreatmentIDs(treatmentIDs []uint, centerIDs []uint) ([]models.Service, error)
	FindAll(beautyCenterID uint, categoryID uint, isActive *bool, page, limit int) ([]models.Service, int64, error)
}
