package repository

import "github.com/Nina-99/yottu-api/models"

type PersonnelInvitationRepository interface {
	Create(invitation *models.PersonnelInvitation) error
	Update(invitation *models.PersonnelInvitation) error
	FindByToken(token string) (*models.PersonnelInvitation, error)
	FindByBeautyCenterID(beautyCenterID uint) ([]models.PersonnelInvitation, error)
	FindByEmailAndCenter(email string, beautyCenterID uint) (*models.PersonnelInvitation, error)
}
