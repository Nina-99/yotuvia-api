package configs

import (
	"fmt"
	"os"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func DBConnection() {
	connectionStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"))

	db, err := gorm.Open(postgres.Open(connectionStr), &gorm.Config{})

	if err != nil {
		utils.Log.Fatal("Failed to connect to database: ", err)
	}

	utils.Log.Info("Connected Successfully to the database")

	list_models := []interface{}{
		&models.AuthAccount{},
		&models.AuthToken{},
		&models.BeautyCenter{},
		&models.BeautyCenterImage{},
		&models.Category{},
		&models.TreatmentCategory{},
		&models.Treatment{},
		&models.TokenAccount{},
		&models.TokenTopUp{},
		&models.TokenTransaction{},
		&models.TokenMission{},
		&models.TokenMissionProgress{},
		&models.Referral{},
		&models.Client{},
		&models.CustomerPunchCard{},
		&models.Favorite{},
		&models.PersonnelFollow{},
		&models.FeedPost{},
		&models.LoyaltyProgram{},
		&models.LoyaltyRedemption{},
		&models.Personnel{},
		&models.PersonnelImage{},
		&models.PersonnelService{},
		&models.PostLike{},
		&models.PunchHistory{},
		&models.Reservation{},
		&models.ReservationService{},
		&models.Review{},
		&models.Service{},
		&models.SocialMedia{},
		&models.WorkingHour{},
		&models.TimeBlock{},
		&models.VisitNote{},
		&models.PersonnelInvitation{},
		&models.Notification{},
		&models.PushDevice{},
		&models.UserSession{},
		&models.MarketConfig{},
		&models.VideoComment{},
		&models.Promotion{},
		&models.PromotionRedemption{},
		&models.PaymentConfig{},
		&models.SavedVideo{},
		&models.VideoWatchEvent{},
		&models.WaitlistEntry{},
		&models.PlatformAdmin{},
		&models.BoostCampaign{},
		&models.BoostImpression{},
		&models.TikTokAccount{},
		&models.TikTokConnectionState{},
	}

	// Rename the legacy currency-rate column BEFORE AutoMigrate so GORM never
	// adds a second, empty column next to it.
	renameYtvColumn(db)

	for _, model := range list_models {
		if err := db.AutoMigrate(model); err != nil {
			utils.Log.Fatalf("Error migrating the model %T: %v", model, err)
		}
	}

	DB = db

	// Period-aware missions: drop the legacy (center, mission) unique index and
	// guarantee the (center, mission, period_key) one before any seeding.
	MigrateMissionPeriods(db)

	SeedDefaultMissions()
	SeedMarketConfigs()
	SeedTreatmentTaxonomy()
	BackfillLegacyVideos()
	SeedDemoCenters()
	// Runs after demo centers so their freshly created services get linked in
	// the same boot. Idempotent and non-destructive: never overwrites a row
	// that already carries a TreatmentID.
	BackfillServiceTreatments()
	// Fills line-level professional attribution for reservations created before
	// ReservationService.PersonnelID existed. Idempotent (only NULL rows) and
	// never fatal on a bad row.
	BackfillReservationServicePersonnel()
	// Copies the legacy platform-wide payment config into one row per centre so
	// the per-centre economy keeps its existing QR/phone/tiers. Non-destructive
	// and idempotent: an already-configured centre is never overwritten and a
	// re-run creates nothing. The legacy singleton row is kept for rollback.
	BackfillPaymentConfigs()
	// Creates the platform-level super_admin from SUPER_ADMIN_EMAIL /
	// SUPER_ADMIN_PASSWORD. Idempotent and non-destructive (an existing hash is
	// never overwritten); a half-configured or unset env is a logged no-op. This
	// is the ONLY way the role comes to exist — registration cannot mint it.
	SeedPlatformAdmin()
}

// renameYtvColumn renames the legacy market_config currency-rate column in
// place. It runs before AutoMigrate so GORM does not add a second, empty
// ytv_to_fiat_rate column next to the old one. No-op on fresh databases.
func renameYtvColumn(db *gorm.DB) {
	model := &models.MarketConfig{}
	if !db.Migrator().HasTable(model) {
		return
	}
	const oldName, newName = "yotuvia_to_fiat_rate", "ytv_to_fiat_rate"
	if db.Migrator().HasColumn(model, oldName) && !db.Migrator().HasColumn(model, newName) {
		if err := db.Migrator().RenameColumn(model, oldName, newName); err != nil {
			utils.Log.Errorf("[MIGRATE] failed to rename market_config.%s: %v", oldName, err)
			return
		}
		utils.Log.Infof("[MIGRATE] renamed market_config.%s → %s", oldName, newName)
	}
}

// MigrateMissionPeriods replaces the legacy (center, mission) uniqueness with
// (center, mission, period_key) so recurring missions can keep one row per
// window. Idempotent: safe on fresh and already-migrated databases.
func MigrateMissionPeriods(db *gorm.DB) {
	model := &models.TokenMissionProgress{}
	if !db.Migrator().HasTable(model) {
		return
	}

	if db.Migrator().HasIndex(model, "idx_mission_center") {
		if err := db.Migrator().DropIndex(model, "idx_mission_center"); err != nil {
			utils.Log.Errorf("[MIGRATE] failed to drop idx_mission_center: %v", err)
		} else {
			utils.Log.Infof("[MIGRATE] dropped legacy idx_mission_center")
		}
	}

	// Existing rows predate periods: they belong to the one-shot mission.
	if err := db.Model(model).
		Where("period_key = ? OR period_key IS NULL", "").
		Update("period_key", "once").Error; err != nil {
		utils.Log.Errorf("[MIGRATE] failed to backfill period_key: %v", err)
	}

	if !db.Migrator().HasIndex(model, "idx_mission_center_period") {
		if err := db.Migrator().CreateIndex(model, "idx_mission_center_period"); err != nil {
			utils.Log.Errorf("[MIGRATE] failed to create idx_mission_center_period: %v", err)
		}
	}
}
