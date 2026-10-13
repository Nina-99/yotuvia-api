package configs

import (
	"fmt"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newPaymentConfigBackfillDB opens an in-memory DB pinned to a single
// connection (a pooled :memory: database gives each connection its own empty
// schema) and migrates the two tables the backfill touches.
func newPaymentConfigBackfillDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.PaymentConfig{}, &models.BeautyCenter{}))
	return db
}

func seedBackfillCenter(t *testing.T, db *gorm.DB, id uint) {
	t.Helper()
	require.NoError(t, db.Create(&models.BeautyCenter{
		ID:           id,
		Name:         "Backfill Center",
		Email:        fmt.Sprintf("backfill%d@test.com", id),
		Phone:        "5550000",
		ReferralCode: fmt.Sprintf("BF%03d", id),
	}).Error)
}

// TestBackfillPaymentConfigs_CopiesLegacyValuesExactlyOnce pins both acceptance
// properties: every existing centre gets the legacy QR/phone/tiers, and a second
// run creates nothing (idempotent).
func TestBackfillPaymentConfigs_CopiesLegacyValuesExactlyOnce(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := newPaymentConfigBackfillDB(t)
	DB = db

	require.NoError(t, db.Create(&models.PaymentConfig{
		BeautyCenterID: 0, // legacy singleton marker
		PaymentQRURL:   "legacy-qr",
		PaymentPhone:   "legacy-phone",
		TiersJSON:      `{"USD":[{"min":50,"bonusPct":7}]}`,
	}).Error)
	seedBackfillCenter(t, db, 1)
	seedBackfillCenter(t, db, 2)

	first := BackfillPaymentConfigs()
	assert.True(t, first.HasLegacyData)
	assert.Equal(t, 2, first.Centers)
	assert.Equal(t, 2, first.Created)
	assert.Equal(t, 0, first.Skipped)

	for _, centreID := range []uint{1, 2} {
		var cfg models.PaymentConfig
		require.NoError(t, db.Where("beauty_center_id = ?", centreID).First(&cfg).Error)
		assert.Equal(t, "legacy-qr", cfg.PaymentQRURL, "centre %d gets the legacy QR", centreID)
		assert.Equal(t, "legacy-phone", cfg.PaymentPhone)
		assert.Equal(t, `{"USD":[{"min":50,"bonusPct":7}]}`, cfg.TiersJSON)
	}

	// Second run: every centre is already configured, so nothing is created and
	// the copied values are untouched.
	second := BackfillPaymentConfigs()
	assert.Equal(t, 0, second.Created)
	assert.Equal(t, 2, second.Skipped)

	var count int64
	require.NoError(t, db.Model(&models.PaymentConfig{}).
		Where("beauty_center_id IN ?", []uint{1, 2}).Count(&count).Error)
	assert.Equal(t, int64(2), count, "no duplicate rows on a re-run")

	// The legacy row is KEPT (not soft-deleted) so the platform-wide values stay
	// available for rollback, and no stray rows were created.
	var legacyKept models.PaymentConfig
	require.NoError(t, db.Where("beauty_center_id = ?", 0).First(&legacyKept).Error)
	assert.Equal(t, "legacy-qr", legacyKept.PaymentQRURL, "legacy row is preserved")

	var total int64
	require.NoError(t, db.Model(&models.PaymentConfig{}).Count(&total).Error)
	assert.Equal(t, int64(3), total, "legacy row + one row per centre, no duplicates")
}

// TestBackfillPaymentConfigs_DoesNotOverwriteExistingCentreConfig proves the
// operation is non-destructive: a centre that already owns a config keeps it.
func TestBackfillPaymentConfigs_DoesNotOverwriteExistingCentreConfig(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := newPaymentConfigBackfillDB(t)
	DB = db

	require.NoError(t, db.Create(&models.PaymentConfig{
		BeautyCenterID: 0,
		PaymentQRURL:   "legacy-qr",
		PaymentPhone:   "legacy-phone",
		TiersJSON:      `{"USD":[{"min":50,"bonusPct":7}]}`,
	}).Error)
	require.NoError(t, db.Create(&models.PaymentConfig{
		BeautyCenterID: 1,
		PaymentQRURL:   "own-qr",
		PaymentPhone:   "own-phone",
		TiersJSON:      `{"USD":[{"min":1,"bonusPct":1}]}`,
	}).Error)
	seedBackfillCenter(t, db, 1)
	seedBackfillCenter(t, db, 2)

	stats := BackfillPaymentConfigs()
	assert.Equal(t, 2, stats.Centers)
	assert.Equal(t, 1, stats.Created, "only the unconfigured centre is created")
	assert.Equal(t, 1, stats.Skipped, "the configured centre is skipped")

	var one models.PaymentConfig
	require.NoError(t, db.Where("beauty_center_id = ?", 1).First(&one).Error)
	assert.Equal(t, "own-qr", one.PaymentQRURL, "an owned config is never overwritten")
	assert.Equal(t, "own-phone", one.PaymentPhone)

	var two models.PaymentConfig
	require.NoError(t, db.Where("beauty_center_id = ?", 2).First(&two).Error)
	assert.Equal(t, "legacy-qr", two.PaymentQRURL)
}

// TestBackfillPaymentConfigs_NoLegacyDataIsNoOp proves a fresh install (no
// legacy values) creates no rows, leaving lazy create-if-missing to handle new
// centres.
func TestBackfillPaymentConfigs_NoLegacyDataIsNoOp(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := newPaymentConfigBackfillDB(t)
	DB = db

	// Legacy row present but empty.
	require.NoError(t, db.Create(&models.PaymentConfig{BeautyCenterID: 0}).Error)
	seedBackfillCenter(t, db, 1)

	stats := BackfillPaymentConfigs()
	assert.False(t, stats.HasLegacyData)
	assert.Equal(t, 0, stats.Created)

	var count int64
	require.NoError(t, db.Model(&models.PaymentConfig{}).
		Where("beauty_center_id = ?", 1).Count(&count).Error)
	assert.Equal(t, int64(0), count, "an empty legacy row must not spawn rows")
}
