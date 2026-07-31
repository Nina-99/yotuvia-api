package repository

import "github.com/Nina-99/yottu-api/models"

type PersonnelServiceRepository interface {
	Create(ps *models.PersonnelService) error
	Delete(id uint) error
	FindByID(id uint) (*models.PersonnelService, error)
	FindByPersonnelID(personnelID uint) ([]models.PersonnelService, error)
	FindByServiceID(serviceID uint) ([]models.PersonnelService, error)
	FindByPersonnelAndService(personnelID, serviceID uint) (*models.PersonnelService, error)
}
