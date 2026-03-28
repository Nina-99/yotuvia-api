package repository

import "github.com/Nina-99/yottu-api/models"

type SocialMediaRepository interface {
	Create(socialMedia *models.SocialMedia) error
	Update(socialMedia *models.SocialMedia) error
	Delete(socialMedia *models.SocialMedia) error
	FindByID(id uint) (*models.SocialMedia, error)
	FindByBeautyCenterID(beautyCenterID uint) (*models.SocialMedia, error)
}
