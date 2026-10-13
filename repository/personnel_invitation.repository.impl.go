package repository

import (
	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type PersonnelInvitationRepositoryImpl struct {
	db *gorm.DB
}

func NewPersonnelInvitationRepositoryImpl(db *gorm.DB) PersonnelInvitationRepository {
	return &PersonnelInvitationRepositoryImpl{db: db}
}

func (r *PersonnelInvitationRepositoryImpl) Create(invitation *models.PersonnelInvitation) error {
	return r.db.Create(invitation).Error
}

func (r *PersonnelInvitationRepositoryImpl) Update(invitation *models.PersonnelInvitation) error {
	return r.db.Save(invitation).Error
}

func (r *PersonnelInvitationRepositoryImpl) FindByToken(token string) (*models.PersonnelInvitation, error) {
	var invitation models.PersonnelInvitation
	err := r.db.Where("token = ?", token).First(&invitation).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &invitation, nil
}

func (r *PersonnelInvitationRepositoryImpl) FindByBeautyCenterID(beautyCenterID uint) ([]models.PersonnelInvitation, error) {
	var invitations []models.PersonnelInvitation
	err := r.db.Where("beauty_center_id = ?", beautyCenterID).Order("created_at DESC").Find(&invitations).Error
	return invitations, err
}

func (r *PersonnelInvitationRepositoryImpl) FindByEmailAndCenter(email string, beautyCenterID uint) (*models.PersonnelInvitation, error) {
	var invitation models.PersonnelInvitation
	err := r.db.Where("email = ? AND beauty_center_id = ? AND status = 'pending'", email, beautyCenterID).First(&invitation).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &invitation, nil
}
