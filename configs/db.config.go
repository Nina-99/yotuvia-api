package configs

import (
	"fmt"
	"os"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/utils"
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
	}

	for _, model := range list_models {
		if err := db.AutoMigrate(model); err != nil {
			utils.Log.Fatalf("Error migrating the model %T: %v", model, err)
		}
	}

	DB = db

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
}
