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

	if err := db.AutoMigrate(&models.AuthAccount{}); err != nil {
		log.Fatal("Falied to Migrate of Table AuthAccount", err)
	}
	if err := db.AutoMigrate(&models.BeautyCenter{}); err != nil {
		log.Fatal("Falied to Migrate of Table BeautyCenter", err)
	}
	if err := db.AutoMigrate(&models.BeautyCenterImage{}); err != nil {
		log.Fatal("Falied to Migrate of Table BeautyCenterImage", err)
	}
	if err := db.AutoMigrate(&models.Category{}); err != nil {
		log.Fatal("Falied to Migrate of Table Category", err)
	}
	if err := db.AutoMigrate(&models.Client{}); err != nil {
		if err := db.AutoMigrate(&models.Favorite{}); err != nil {
			log.Fatal("Falied to Migrate of Table Favorite", err)
		}
		log.Fatal("Falied to Migrate of Table Client", err)
	}
	if err := db.AutoMigrate(&models.Membership{}); err != nil {
		log.Fatal("Falied to Migrate of Table Membership", err)
	}
	if err := db.AutoMigrate(&models.Personnel{}); err != nil {
		log.Fatal("Falied to Migrate of Table Personnel", err)
	}
	if err := db.AutoMigrate(&models.PersonnelImage{}); err != nil {
		log.Fatal("Falied to Migrate of Table PersonnelImage", err)
	}
	if err := db.AutoMigrate(&models.Reservation{}); err != nil {
		log.Fatal("Falied to Migrate of Table Reservation", err)
	}
	if err := db.AutoMigrate(&models.Service{}); err != nil {
		log.Fatal("Falied to Migrate of Table Service", err)
	}
	if err := db.AutoMigrate(&models.Review{}); err != nil {
		log.Fatal("Falied to Migrate of Table Review", err)
	}

	if err := db.AutoMigrate(&models.SocialMedia{}); err != nil {
		log.Fatal("Falied to Migrate of Table SocialMedia", err)
	}
	if err := db.AutoMigrate(&models.WorkingHour{}); err != nil {
		log.Fatal("Falied to Migrate of Table WorkingHour", err)
	}
	if err := db.AutoMigrate(&models.FeedPost{}); err != nil {
		log.Fatal("Falied to Migrate of Table FeedPost", err)
	}
	if err := db.AutoMigrate(&models.PostLike{}); err != nil {
		log.Fatal("Falied to Migrate of Table PostLike", err)
	}

	DB = db
}
