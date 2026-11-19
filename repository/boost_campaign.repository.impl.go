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

func (r *BoostCampaignRepositoryImpl) FindEligible(now time.Time, limit int) ([]EligibleBoost, error) {
	var rows []EligibleBoost
	err := r.Db.
		Model(&models.BoostCampaign{}).
		// LEFT JOIN on purpose: a campaign whose centre row is missing (or was
		// soft-deleted) must still be auctioned exactly as before. Only the geo
		// filter depends on the coordinates, and COALESCE turns "unknown" into 0,
		// which the service treats as "no location" rather than as a position.
		Select("boost_campaigns.*, COALESCE(bc.latitude, 0) AS center_latitude, COALESCE(bc.longitude, 0) AS center_longitude").
		Joins("LEFT JOIN beauty_centers bc ON bc.id = boost_campaigns.beauty_center_id AND bc.deleted_at IS NULL").
		Where("boost_campaigns.status = ? AND boost_campaigns.starts_at <= ? AND boost_campaigns.ends_at >= ? "+
			"AND boost_campaigns.spent_milli < boost_campaigns.budget_milli",
			"active", now, now).
		Order("boost_campaigns.bid_cpm_milli DESC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
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

// CountImpressionsByViewer groups the viewer's billed impressions for the given
// campaigns in ONE query. An empty id set returns an empty map (no query).
func (r *BoostCampaignRepositoryImpl) CountImpressionsByViewer(viewerID uint, campaignIDs []uint) (map[uint]int64, error) {
	counts := make(map[uint]int64, len(campaignIDs))
	if viewerID == 0 || len(campaignIDs) == 0 {
		return counts, nil
	}

	type row struct {
		CampaignID uint
		Total      int64
	}
	var rows []row
	err := r.Db.Model(&models.BoostImpression{}).
		Select("campaign_id, COUNT(*) AS total").
		Where("viewer_id = ? AND campaign_id IN ?", viewerID, campaignIDs).
		Group("campaign_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r2 := range rows {
		counts[r2.CampaignID] = r2.Total
	}
	return counts, nil
}

// AddUnservedWins books delivery misses in ONE statement. Called for campaigns
// that won an auction but had no renderable post to attach to.
func (r *BoostCampaignRepositoryImpl) AddUnservedWins(campaignIDs []uint) error {
	if len(campaignIDs) == 0 {
		return nil
	}
	return r.Db.Model(&models.BoostCampaign{}).
		Where("id IN ?", campaignIDs).
		UpdateColumn("unserved_wins", gorm.Expr("unserved_wins + 1")).Error
}

// DailyImpressions groups the campaign's billed views per calendar day.
//
// The spend is derived here, not read from the campaign: each impression stores
// the second price it was billed at, so the day's cost is the sum of
// priceCPM/1000 over that day's views. SQLite and Postgres both accept the
// substr/date split through GORM's date() — we use substr on the ISO timestamp
// so the query stays dialect-neutral.
func (r *BoostCampaignRepositoryImpl) DailyImpressions(campaignID uint) ([]BoostDailyRow, error) {
	var rows []BoostDailyRow

	dayExpr := "substr(created_at, 1, 10)"
	if r.Db.Dialector.Name() == "postgres" {
		dayExpr = "to_char(created_at, 'YYYY-MM-DD')"
	}

	err := r.Db.Model(&models.BoostImpression{}).
		Select(dayExpr+" AS day, COUNT(*) AS impressions, "+
			"CAST(COALESCE(SUM(price_cpm_milli), 0) / 1000 AS BIGINT) AS spend_milli").
		Where("campaign_id = ?", campaignID).
		Group(dayExpr).
		Order("day ASC").
		Scan(&rows).Error
	return rows, err
}

// PlatformAggregates sums the ad book in one query. Money is summed in milli so
// the caller converts once.
func (r *BoostCampaignRepositoryImpl) PlatformAggregates() (BoostAggregates, error) {
	var agg BoostAggregates

	err := r.Db.Model(&models.BoostCampaign{}).
		Select(`COUNT(*) AS campaigns,
			COALESCE(SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END), 0) AS active_campaigns,
			COALESCE(SUM(impressions), 0) AS impressions,
			COALESCE(SUM(spent_milli), 0) AS spent_milli,
			COALESCE(SUM(unserved_wins), 0) AS unserved_wins`).
		Scan(&agg).Error
	return agg, err
}

// DailyAllImpressions is the platform-wide delivery curve.
func (r *BoostCampaignRepositoryImpl) DailyAllImpressions() ([]BoostDailyRow, error) {
	var rows []BoostDailyRow

	dayExpr := "substr(created_at, 1, 10)"
	if r.Db.Dialector.Name() == "postgres" {
		dayExpr = "to_char(created_at, 'YYYY-MM-DD')"
	}

	err := r.Db.Model(&models.BoostImpression{}).
		Select(dayExpr + " AS day, COUNT(*) AS impressions, " +
			"CAST(COALESCE(SUM(price_cpm_milli), 0) / 1000 AS BIGINT) AS spend_milli").
		Group(dayExpr).
		Order("day ASC").
		Scan(&rows).Error
	return rows, err
}

// TopBySpend lists the biggest advertisers. LEFT JOIN so a campaign whose centre
// vanished still appears (with an empty name) instead of disappearing from the
// platform's revenue view.
func (r *BoostCampaignRepositoryImpl) TopBySpend(limit int) ([]BoostTopCampaign, error) {
	if limit <= 0 {
		limit = 10
	}
	var rows []BoostTopCampaign
	err := r.Db.Model(&models.BoostCampaign{}).
		Select(`boost_campaigns.id AS campaign_id,
			boost_campaigns.beauty_center_id AS center_id,
			COALESCE(bc.name, '') AS center_name,
			boost_campaigns.spent_milli AS spent_milli,
			boost_campaigns.impressions AS impressions,
			boost_campaigns.unserved_wins AS unserved_wins`).
		Joins("LEFT JOIN beauty_centers bc ON bc.id = boost_campaigns.beauty_center_id").
		Where("boost_campaigns.spent_milli > 0").
		Order("boost_campaigns.spent_milli DESC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}
