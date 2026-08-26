package configs

import (
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/utils"
)

func SeedDefaultMissions() {
	missions := []models.TokenMission{
		{
			Name:         "First Completed Reservation",
			Description:  "Complete your first reservation",
			TriggerType:  "reservation_completed",
			TriggerCount: 1,
			RewardTokens: 10000,
			Icon:         "🎯",
			IsActive:     true,
			SortOrder:    1,
		},
		{
			Name:         "Loyalty Achiever",
			Description:  "Complete 50 reservations in a month",
			TriggerType:  "reservation_completed",
			TriggerCount: 50,
			RewardTokens: 25000,
			Icon:         "🏆",
			IsActive:     true,
			SortOrder:    2,
		},
		{
			Name:         "Stamp Promoter",
			Description:  "Activate a stamp card promotion",
			TriggerType:  "stamp_activated",
			TriggerCount: 1,
			RewardTokens: 15000,
			Icon:         "🎟️",
			IsActive:     true,
			SortOrder:    3,
		},
		{
			Name:         "Profile Star",
			Description:  "Complete your profile with photo and description",
			TriggerType:  "profile_completed",
			TriggerCount: 1,
			RewardTokens: 5000,
			Icon:         "⭐",
			IsActive:     true,
			SortOrder:    4,
		},
		{
			Name:         "Receive 5 Reviews",
			Description:  "Receive 5 reviews from your clients",
			TriggerType:  "review_received",
			TriggerCount: 5,
			RewardTokens: 3000,
			Icon:         "💬",
			IsActive:     true,
			SortOrder:    5,
		},
		{
			Name:         "10 Completed Reservations",
			Description:  "Complete 10 reservations",
			TriggerType:  "reservation_completed",
			TriggerCount: 10,
			RewardTokens: 6000,
			Icon:         "🥉",
			IsActive:     true,
			SortOrder:    6,
		},
		{
			Name:         "Refer a Colleague",
			Description:  "Refer a colleague whose center gets verified",
			TriggerType:  "referral",
			TriggerCount: 1,
			RewardTokens: 30000,
			Icon:         "🤝",
			IsActive:     true,
			SortOrder:    7,
		},
	}

	// feat-token-missions D3: review responses don't exist as a product
	// surface yet — the legacy trigger type is deactivated so stale rows on
	// pre-existing databases never render or progress.
	if err := DB.Model(&models.TokenMission{}).
		Where("trigger_type = ?", "review_responded").
		Update("is_active", false).Error; err != nil {
		utils.Log.Errorf("[SEED] Failed to deactivate legacy review_responded mission: %v", err)
	}

	for _, mission := range missions {
		var existing models.TokenMission
		result := DB.Where("trigger_type = ? AND trigger_count = ?", mission.TriggerType, mission.TriggerCount).First(&existing)
		if result.Error != nil {
			if err := DB.Create(&mission).Error; err != nil {
				utils.Log.Errorf("[SEED] Failed to create mission %s: %v", mission.Name, err)
			} else {
				utils.Log.Infof("[SEED] Created mission: %s (+%.0f YOTTUs)", mission.Name, utils.FromMilli(mission.RewardTokens))
			}
		} else if existing.Name != mission.Name ||
			existing.Description != mission.Description ||
			existing.Icon != mission.Icon ||
			existing.SortOrder != mission.SortOrder ||
			existing.RewardTokens != mission.RewardTokens ||
			!existing.IsActive {
			// Self-heal drifted legacy rows (e.g. reward loaded as whole units
			// instead of milli-YOTTU) back to the canonical definition.
			if err := DB.Model(&existing).Updates(map[string]interface{}{
				"name":          mission.Name,
				"description":   mission.Description,
				"icon":          mission.Icon,
				"sort_order":    mission.SortOrder,
				"reward_tokens": mission.RewardTokens,
				"is_active":     true,
			}).Error; err != nil {
				utils.Log.Errorf("[SEED] Failed to heal mission %s: %v", mission.Name, err)
			} else {
				utils.Log.Infof("[SEED] Healed drifted mission %s → %.0f YOTTUs", mission.Name, utils.FromMilli(mission.RewardTokens))
			}
		}
	}
}

// SeedMarketConfigs seeds the per-currency token economy configuration
// (feat-token-economy-config) with the same create-or-heal pattern as
// missions: missing rows are created, drifted rows are restored to canonical.
func SeedMarketConfigs() {
	configs := []models.MarketConfig{
		// Bolivia — launch market. 1 YOTTU = 1 BOB; commission 3.5%;
		// floor 0.35 YOTTU (= commission of a 10 BOB reference ticket).
		{
			Currency:        "BOB",
			YottuToFiatRate: 1.0,
			CommissionPct:   3.5,
			MinChargeMilli:  350,
		},
	}

	for _, cfg := range configs {
		var existing models.MarketConfig
		result := DB.Where("currency = ?", cfg.Currency).First(&existing)
		if result.Error != nil {
			if err := DB.Create(&cfg).Error; err != nil {
				utils.Log.Errorf("[SEED] Failed to create market config %s: %v", cfg.Currency, err)
			} else {
				utils.Log.Infof("[SEED] Created market config %s: %.2f%% / floor %.2f YOTTUs",
					cfg.Currency, cfg.CommissionPct, utils.FromMilli(cfg.MinChargeMilli))
			}
		} else if existing.YottuToFiatRate != cfg.YottuToFiatRate ||
			existing.CommissionPct != cfg.CommissionPct ||
			existing.MinChargeMilli != cfg.MinChargeMilli {
			if err := DB.Model(&existing).Updates(map[string]interface{}{
				"yottu_to_fiat_rate": cfg.YottuToFiatRate,
				"commission_pct":     cfg.CommissionPct,
				"min_charge_milli":   cfg.MinChargeMilli,
			}).Error; err != nil {
				utils.Log.Errorf("[SEED] Failed to heal market config %s: %v", cfg.Currency, err)
			} else {
				utils.Log.Infof("[SEED] Healed drifted market config %s → %.2f%% / floor %.2f",
					cfg.Currency, cfg.CommissionPct, utils.FromMilli(cfg.MinChargeMilli))
			}
		}
	}
}

// BackfillLegacyVideos migrates legacy `videos` rows into feed_posts and then
// drops the legacy table (feat-video-feed-unification D1/D7). No-op when the
// legacy table is absent. Invalid rows are log-and-skip — the batch never
// aborts. PR body documents DB-backup expectation for rollback.
func BackfillLegacyVideos() {
	if !DB.Migrator().HasTable("videos") {
		return
	}

	type legacyRow struct {
		BeautyCenterID uint
		Title          string
		Description    string
		FilePath       string
		ThumbnailURL   string
		Views          int
	}

	var legacy []legacyRow
	if err := DB.Table("videos").
		Select("beauty_center_id, title, description, file_path, thumbnail_url, views").
		Where("deleted_at IS NULL").
		Scan(&legacy).Error; err != nil {
		utils.Log.Errorf("[SEED] legacy videos backfill read failed: %v", err)
		return
	}
	if len(legacy) == 0 {
		utils.Log.Infof("[SEED] legacy videos table empty — nothing to backfill")
		return
	}

	migrated := 0
	for _, row := range legacy {
		if row.FilePath == "" || row.BeautyCenterID == 0 {
			utils.Log.Warnf("[SEED] skipping invalid legacy video (center %d): %q", row.BeautyCenterID, row.Title)
			continue
		}
		caption := row.Title
		if len(caption) < 3 && len(row.Description) <= 255 {
			caption = row.Description
		}
		post := models.FeedPost{
			BeautyCenterID: row.BeautyCenterID,
			VideoURL:       row.FilePath,
			ThumbnailURL:   row.ThumbnailURL,
			Caption:        caption,
			ViewsCount:     row.Views,
			IsActive:       true,
		}
		if err := DB.Create(&post).Error; err != nil {
			utils.Log.Errorf("[SEED] failed to migrate legacy video %q: %v", row.Title, err)
			continue
		}
		migrated++
	}

	if err := DB.Migrator().DropTable("videos"); err != nil {
		utils.Log.Errorf("[SEED] backfill complete but failed to drop legacy videos table: %v", err)
		return
	}
	utils.Log.Infof("[SEED] Backfilled %d/%d legacy videos into feed_posts; legacy table dropped", migrated, len(legacy))
}
