package repository

import "github.com/Nina-99/yottu-api/models"

type PersonnelRepository interface {
	Create(personnel *models.Personnel) error
	Update(personnel *models.Personnel) error
	Delete(id uint) error
	FindByID(id uint) (*models.Personnel, error)
	FindAll(beautyCenterID uint) ([]models.Personnel, error)
	FindByEmail(email string) (*models.Personnel, error)
}
