package repository

import "github.com/Nina-99/yottu-api/models"

type CenterMembershipRepository interface {
	Save(tx *models.CenterMembership) error
	UpdateStatus(centerMembershipID uint, status string) error
	GetLatestByCenterID(centerID uint) (*models.CenterMembership, error)
	FindByID(id uint) (*models.CenterMembership, error)
}
