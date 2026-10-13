package configs

import (
	"errors"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"gorm.io/gorm"
)

// paymentConfigLegacyCenterID marks the pre-multi-tenant singleton payment
// config row. Public reads reject any centre id <= 0 and admin writes are
// claim-scoped to a real centre, so this row is never served; it is kept (not
// soft-deleted) so the platform-wide values remain available for rollback.
const paymentConfigLegacyCenterID uint = 0

// PaymentConfigBackfillStats reports the outcome of the per-centre payment
// config backfill.
type PaymentConfigBackfillStats struct {
	// Centers is the number of beauty centres considered.
	Centers int
	// Created is the number of per-centre config rows created by this run.
	Created int
	// Skipped counts centres already owning a config row (a re-run skips all).
	Skipped int
	// HasLegacyData is true when the legacy singleton row carried a QR, phone
	// or tier ladder worth copying. When false the whole run is a no-op and no
	// rows are created.
	HasLegacyData bool
}

// repairPaymentConfigSequence re-syncs the table's id sequence with its maximum
// id, on PostgreSQL only.
//
// Why it is needed: the pre-multi-tenant singleton code inserted its row with an
// EXPLICIT id (ID = 1), so `payment_configs_id_seq` was never advanced. GORM
// omits a zero auto-increment primary key, so the next insert asked the
// sequence for id = 1 and collided with that existing row:
//
//	ERROR: duplicate key value violates unique constraint "payment_configs_pkey"
//
// Every database created before the per-centre change is in that state, so the
// per-centre config could not be created at all. setval is idempotent, and the
// DO block tolerates a column that is not backed by a sequence.
func repairPaymentConfigSequence() {
	if DB == nil || DB.Dialector.Name() != "postgres" {
		return
	}

	const stmt = `
DO $$
DECLARE seq text;
BEGIN
  seq := pg_get_serial_sequence('payment_configs', 'id');
  IF seq IS NOT NULL THEN
    PERFORM setval(seq, COALESCE((SELECT MAX(id) FROM payment_configs), 0) + 1, false);
  END IF;
END $$;`

	if err := DB.Exec(stmt).Error; err != nil {
		utils.Log.Errorf("[SEED] Payment config sequence repair failed: %v", err)
		return
	}
	utils.Log.Infof("[SEED] Payment config id sequence re-synced")
}

// BackfillPaymentConfigs copies the legacy platform-wide payment config (the old
// singleton row, BeautyCenterID = 0) into one row per existing beauty centre.
// It follows the BackfillServiceTreatments pattern:
//
//   - it NEVER overwrites a centre that already owns a config row, so the
//     operation is non-destructive and safe to re-run;
//   - it only creates rows when the legacy row actually holds data, so a fresh
//     install (no legacy row) is a no-op and lazy create-if-missing handles new
//     centres instead;
//   - it never drops, truncates or deletes anything.
//
// Idempotency: the second run finds every centre already configured and creates
// zero rows, so the legacy values are copied exactly once.
func BackfillPaymentConfigs() PaymentConfigBackfillStats {
	var stats PaymentConfigBackfillStats

	// Repair FIRST, before any early return: a migrated database whose legacy
	// row holds no data still needs the sequence fixed, otherwise the per-centre
	// rows can never be inserted.
	repairPaymentConfigSequence()

	var legacy models.PaymentConfig
	err := DB.Where("beauty_center_id = ?", paymentConfigLegacyCenterID).First(&legacy).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Log.Errorf("[SEED] Payment config backfill: could not read legacy row: %v", err)
		}
		utils.Log.Infof("[SEED] Payment config backfill: no legacy config row, nothing to copy")
		return stats
	}

	if legacy.PaymentQRURL == "" && legacy.PaymentPhone == "" && legacy.TiersJSON == "" {
		utils.Log.Infof("[SEED] Payment config backfill: legacy row holds no data, nothing to copy")
		return stats
	}
	stats.HasLegacyData = true

	var centres []models.BeautyCenter
	if err := DB.Select("id").Find(&centres).Error; err != nil {
		utils.Log.Errorf("[SEED] Payment config backfill: could not load centres: %v", err)
		return stats
	}
	stats.Centers = len(centres)

	// One query for the centres that already own a config, so the per-centre
	// check below is a map lookup and a re-run is a cheap no-op.
	var configured []uint
	if err := DB.Model(&models.PaymentConfig{}).
		Where("beauty_center_id > ?", paymentConfigLegacyCenterID).
		Pluck("beauty_center_id", &configured).Error; err != nil {
		utils.Log.Errorf("[SEED] Payment config backfill: could not read configured centres: %v", err)
		return stats
	}
	alreadyConfigured := make(map[uint]struct{}, len(configured))
	for _, id := range configured {
		alreadyConfigured[id] = struct{}{}
	}

	for _, centre := range centres {
		if centre.ID == paymentConfigLegacyCenterID {
			continue
		}
		if _, ok := alreadyConfigured[centre.ID]; ok {
			stats.Skipped++
			continue
		}
		cfg := models.PaymentConfig{
			BeautyCenterID: centre.ID,
			PaymentQRURL:   legacy.PaymentQRURL,
			PaymentPhone:   legacy.PaymentPhone,
			TiersJSON:      legacy.TiersJSON,
		}
		if err := DB.Create(&cfg).Error; err != nil {
			stats.Skipped++
			utils.Log.Errorf("[SEED] Payment config backfill: failed to configure centre %d: %v", centre.ID, err)
			continue
		}
		stats.Created++
	}

	utils.Log.Infof("[SEED] Payment config backfill: configured %d/%d centres (%d already had a config)",
		stats.Created, stats.Centers, stats.Skipped)
	return stats
}
