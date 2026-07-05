package repository

import "github.com/Nina-99/yottu-api/models"

type MembershipRepository interface {
	Create(membership *models.Membership) error
	Update(membership *models.Membership) error
	Delete(id uint) error
	FindByID(id uint) (*models.Membership, error)
	FindAll(page, limit int) ([]models.Membership, int64, error)
	FindByBeautyCenterID(beautyCenterID uint) ([]models.Membership, error)
}
