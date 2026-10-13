package repository

import "github.com/Nina-99/yotuvia-api/models"

type PersonnelImageRepository interface {
	Save(personnelImage *models.PersonnelImage) error
	FindByID(id uint) (*models.PersonnelImage, error)
	FindByPersonnelID(personnelID uint) ([]models.PersonnelImage, error)
	Delete(id uint) error
}
