package repository

import (
	"errors"

	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type CenterMembershipRepositoryImpl struct {
	db *gorm.DB
}

func NewCenterMembershipRepositoryImpl(db *gorm.DB) CenterMembershipRepository {
	return &CenterMembershipRepositoryImpl{db: db}
}

func (r *CenterMembershipRepositoryImpl) Save(tx *models.CenterMembership) error {
	return r.db.Create(tx).Error
}

func (r *CenterMembershipRepositoryImpl) UpdateStatus(centerMembershipID uint, status string) error {
	return r.db.Model(&models.CenterMembership{}).Where("id = ?", centerMembershipID).Update("status", status).Error
}

func (r *CenterMembershipRepositoryImpl) GetLatestByCenterID(centerID uint) (*models.CenterMembership, error) {
	var membership models.CenterMembership
	err := r.db.Where("beauty_center_id = ?", centerID).Order("created_at desc").First(&membership).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// If they never bought a membership, we return nil, no error
			return nil, nil
		}
		return nil, err
	}
	return &membership, nil
}

func (r *CenterMembershipRepositoryImpl) FindByID(id uint) (*models.CenterMembership, error) {
	var membership models.CenterMembership
	err := r.db.First(&membership, id).Error
	if err != nil {
		return nil, err
	}
	return &membership, nil
}
