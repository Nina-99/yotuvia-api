package tests

import (
	"testing"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupImageTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	err = db.AutoMigrate(&models.BeautyCenter{}, &models.Personnel{}, &models.PersonnelImage{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	bc := models.BeautyCenter{
		Name: "Test Center",
	}
	db.Create(&bc)

	p := models.Personnel{
		BeautyCenterID: bc.ID,
		FirstName:      "John",
		LastName:       "Doe",
		Email:          "john@example.com",
	}
	db.Create(&p)

	return db
}

func TestPersonnelImageRepository(t *testing.T) {
	db := setupImageTestDB(t)
	repo := repository.NewPersonnelImageRepositoryImpl(db)

	var personnelID uint = 1
	var imageID uint

	t.Run("Create Image", func(t *testing.T) {
		img := models.PersonnelImage{
			PersonnelID: personnelID,
			ImageUrl:    "http://example.com/test.jpg",
			IsCover:     true,
			Position:    1,
		}

		err := repo.Save(&img)
		assert.NoError(t, err)
		assert.NotZero(t, img.ID)
		imageID = img.ID
	})

	t.Run("Find Image By ID", func(t *testing.T) {
		img, err := repo.FindByID(imageID)
		assert.NoError(t, err)
		assert.Equal(t, imageID, img.ID)
		assert.Equal(t, "http://example.com/test.jpg", img.ImageUrl)
	})

	t.Run("Find Images By Personnel ID", func(t *testing.T) {
		imgs, err := repo.FindByPersonnelID(personnelID)
		assert.NoError(t, err)
		assert.Len(t, imgs, 1)
		assert.Equal(t, imageID, imgs[0].ID)
	})

	t.Run("Delete Image", func(t *testing.T) {
		err := repo.Delete(imageID)
		assert.NoError(t, err)

		_, err = repo.FindByID(imageID)
		assert.Error(t, err)
	})
}
