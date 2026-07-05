package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type MembershipRepositoryImpl struct {
	db *gorm.DB
}

func NewMembershipRepositoryImpl(db *gorm.DB) MembershipRepository {
	return &MembershipRepositoryImpl{db: db}
}

// Create implements MembershipRepository
func (r *MembershipRepositoryImpl) Create(membership *models.Membership) error {
	return r.db.Create(membership).Error
}

// Update implements MembershipRepository
func (r *MembershipRepositoryImpl) Update(membership *models.Membership) error {
	return r.db.Save(membership).Error
}

// Delete implements MembershipRepository
func (r *MembershipRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.Membership{}, id).Error
}

// FindByID implements MembershipRepository
func (r *MembershipRepositoryImpl) FindByID(id uint) (*models.Membership, error) {
	var membership models.Membership
	err := r.db.Preload("BeautyCenter").First(&membership, id).Error
	if err != nil {
		return nil, err
	}
	return &membership, nil
}

// FindAll implements MembershipRepository
func (r *MembershipRepositoryImpl) FindAll(page, limit int) ([]models.Membership, int64, error) {
	var memberships []models.Membership
	var total int64

	r.db.Model(&models.Membership{}).Count(&total)

	offset := (page - 1) * limit
	err := r.db.Preload("BeautyCenter").Offset(offset).Limit(limit).Find(&memberships).Error
	return memberships, total, err
}

// FindByBeautyCenterID implements MembershipRepository
func (r *MembershipRepositoryImpl) FindByBeautyCenterID(beautyCenterID uint) ([]models.Membership, error) {
	var memberships []models.Membership
	err := r.db.Where("beauty_center_id = ?", beautyCenterID).Find(&memberships).Error
	return memberships, err
}
