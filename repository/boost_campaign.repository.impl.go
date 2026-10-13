package repository

import (
	"errors"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type BoostCampaignRepositoryImpl struct {
	Db *gorm.DB
}

func NewBoostCampaignRepositoryImpl(db *gorm.DB) BoostCampaignRepository {
	return &BoostCampaignRepositoryImpl{Db: db}
}

func (r *BoostCampaignRepositoryImpl) Create(campaign *models.BoostCampaign) error {
	return r.Db.Create(campaign).Error
}

func (r *BoostCampaignRepositoryImpl) Update(campaign *models.BoostCampaign) error {
	return r.Db.Save(campaign).Error
}

// Delete is a hard delete, used to roll back a campaign whose budget hold could
// not be debited (so a failed hold never leaves a phantom campaign).
func (r *BoostCampaignRepositoryImpl) Delete(id uint) error {
	return r.Db.Unscoped().Delete(&models.BoostCampaign{}, id).Error
}

func (r *BoostCampaignRepositoryImpl) FindByID(id uint) (*models.BoostCampaign, error) {
	var campaign models.BoostCampaign
	err := r.Db.First(&campaign, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &campaign, nil
}

func (r *BoostCampaignRepositoryImpl) FindByCenter(centerID uint) ([]models.BoostCampaign, error) {
	var campaigns []models.BoostCampaign
	err := r.Db.Where("beauty_center_id = ?", centerID).
		Order("created_at DESC").
		Limit(ListQueryLimit).
		Find(&campaigns).Error
	return campaigns, err
}

func (r *BoostCampaignRepositoryImpl) FindEligible(now time.Time, limit int) ([]models.BoostCampaign, error) {
	var campaigns []models.BoostCampaign
	err := r.Db.
		Where("status = ? AND starts_at <= ? AND ends_at >= ? AND spent_milli < budget_milli",
			"active", now, now).
		Order("bid_cpm_milli DESC").
		Limit(limit).
		Find(&campaigns).Error
	return campaigns, err
}
