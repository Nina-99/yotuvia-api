package repository

import (
	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type PersonnelInvitationRepository interface {
	Create(invitation *models.PersonnelInvitation) error
	Update(invitation *models.PersonnelInvitation) error
	FindByToken(token string) (*models.PersonnelInvitation, error)
	FindByBeautyCenterID(beautyCenterID uint) ([]models.PersonnelInvitation, error)
	FindByEmailAndCenter(email string, beautyCenterID uint) (*models.PersonnelInvitation, error)

	// Transact runs fn inside one DB transaction. The accept flow uses it so
	// claiming the invitation, creating/updating the staff row and linking the
	// local AuthAccount commit or roll back together.
	Transact(fn func(tx *gorm.DB) error) error
}
