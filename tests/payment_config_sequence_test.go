package tests

import (
	"os"
	"testing"

	"github.com/Nina-99/yotuvia-api/configs"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Tests in this package run without main(), so the package-level logger is nil
// and any utils.Log call panics.
func silenceLoggerForTest(t *testing.T) {
	t.Helper()
	previous := utils.Log
	utils.Log = zap.NewNop().Sugar()
	t.Cleanup(func() { utils.Log = previous })
}

// The legacy singleton code inserted its row with an EXPLICIT id (ID = 1), so
// PostgreSQL never advanced `payment_configs_id_seq`. Any later insert that
// relies on the sequence (GORM omits a zero auto-increment primary key) then
// asks for id = 1 and collides with that existing row:
//
//	ERROR: duplicate key value violates unique constraint "payment_configs_pkey"
//
// That is exactly what happened on the first real database migrated to the
// per-centre model: the (empty) legacy row blocked every per-centre create.
// The repair must therefore run even when the legacy row holds no data.
func TestBackfillPaymentConfigs_RepairsSequenceEvenWithEmptyLegacyRow(t *testing.T) {
	silenceLoggerForTest(t)
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.BeautyCenter{}, &models.PaymentConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Legacy singleton row with no data and an explicit id, like production.
	if err := db.Exec(
		`INSERT INTO payment_configs (id, beauty_center_id, payment_qr_url, payment_phone, tiers_json) VALUES (1, 0, '', '', '')`,
	).Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	if err := db.Create(&models.BeautyCenter{Name: "Centre One"}).Error; err != nil {
		t.Fatalf("seed centre: %v", err)
	}

	previous := configs.DB
	configs.DB = db
	t.Cleanup(func() { configs.DB = previous })

	stats := configs.BackfillPaymentConfigs()

	if stats.HasLegacyData {
		t.Errorf("HasLegacyData = true, want false for an empty legacy row")
	}
	if stats.Created != 0 {
		t.Errorf("Created = %d, want 0 (nothing to copy)", stats.Created)
	}
}

// A per-centre config must be creatable through the repository even when the
// sequence is behind, because that is the state every migrated database is in.
func TestBackfillPaymentConfigs_CentreConfigIsCreatableAfterRepair(t *testing.T) {
	silenceLoggerForTest(t)
	if os.Getenv("SKIP_DB_TESTS") != "" {
		t.Skip("SKIP_DB_TESTS set")
	}
	// The real repair targets PostgreSQL; on SQLite there is no sequence to
	// repair, so this test only pins that the code path is safe to run.
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.BeautyCenter{}, &models.PaymentConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	previous := configs.DB
	configs.DB = db
	t.Cleanup(func() { configs.DB = previous })

	if stats := configs.BackfillPaymentConfigs(); stats.Created != 0 {
		t.Fatalf("Created = %d, want 0 on an empty database", stats.Created)
	}
}
