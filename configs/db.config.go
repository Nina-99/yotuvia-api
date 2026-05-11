package configs

import (
	"fmt"
	"log"
	"os"

	"github.com/Nina-99/yottu-api/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func DBConnection() {
	connectionStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"))

	db, err := gorm.Open(postgres.Open(connectionStr), &gorm.Config{})

	if err != nil {
		log.Fatal("Falied to connect of data base")
	}

	fmt.Println("Connected Successfully to the data base")

	list_models := []interface{}{
		&models.AuthAccount{},
		&models.BeautyCenter{},
		&models.BeautyCenterImage{},
		&models.Category{},
		&models.Client{},
		&models.Favorite{},
		&models.FeedPost{},
		&models.Membership{},
		&models.Personnel{},
		&models.PersonnelImage{},
		&models.PostLike{},
		&models.Reservation{},
		&models.Review{},
		&models.Service{},
		&models.SocialMedia{},
		&models.WorkingHour{},
	}

	for _, model := range list_models {
		if err := db.AutoMigrate(model); err != nil {
			log.Fatalf("Error migrating the model %T: %v", model, err)
		}
	}

	DB = db
}
