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
		&models.BeautyCenter{},
		&models.BeautyCenterImage{},
		&models.CenterMembership{},
		&models.Category{},
		&models.Client{},
		&models.CustomerPunchCard{},
		&models.Favorite{},
		&models.FeedPost{},
		&models.LoyaltyProgram{},
		&models.Membership{},
		&models.Personnel{},
		&models.PersonnelImage{},
		&models.PostLike{},
		&models.PunchHistory{},
		&models.Reservation{},
		&models.Review{},
		&models.Service{},
		&models.SocialMedia{},
		&models.WorkingHour{},
	}

	for _, model := range list_models {
		if err := db.AutoMigrate(model); err != nil {
			utils.Log.Fatalf("Error migrating the model %T: %v", model, err)
		}
	}

	DB = db
}
