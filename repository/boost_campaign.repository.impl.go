package repository

import (
	"errors"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// UpdateInTx persists a campaign inside the caller's transaction.
func (r *BoostCampaignRepositoryImpl) UpdateInTx(tx *gorm.DB, campaign *models.BoostCampaign) error {
	return tx.Save(campaign).Error
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

// FindExpired returns active/paused campaigns whose window already closed.
func (r *BoostCampaignRepositoryImpl) FindExpired(now time.Time) ([]models.BoostCampaign, error) {
	var campaigns []models.BoostCampaign
	err := r.Db.
		Where("status IN ? AND ends_at < ?", []string{"active", "paused"}, now).
		Find(&campaigns).Error
	return campaigns, err
}

// AddSpendInTx books impressions with ONE conditional UPDATE: the row is only
// charged while `spent + cost <= budget`, so concurrent settlements can never
// overspend. Returns false when the budget no longer covers the cost.
func (r *BoostCampaignRepositoryImpl) AddSpendInTx(tx *gorm.DB, campaignID uint, impressions, costMilli int64) (bool, error) {
	res := tx.Exec(`UPDATE boost_campaigns
		SET spent_milli = spent_milli + ?, impressions = impressions + ?, updated_at = ?
		WHERE id = ? AND spent_milli + ? <= budget_milli`,
		costMilli, impressions, time.Now(), campaignID, costMilli)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// MarkExhaustedInTx flips an active campaign to exhausted once its spend
// reached the budget.
func (r *BoostCampaignRepositoryImpl) MarkExhaustedInTx(tx *gorm.DB, campaignID uint) error {
	return tx.Exec(`UPDATE boost_campaigns
		SET status = ?, updated_at = ?
		WHERE id = ? AND status = ? AND spent_milli >= budget_milli`,
		"exhausted", time.Now(), campaignID, "active").Error
}

// CreateImpressionInTx inserts the billing dedupe row. A conflicting unique key
// means the view was already billed, so it reports false instead of erroring.
func (r *BoostCampaignRepositoryImpl) CreateImpressionInTx(tx *gorm.DB, impression *models.BoostImpression) (bool, error) {
	res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(impression)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}
