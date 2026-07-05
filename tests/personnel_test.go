package tests

import (
	"testing"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// AutoMigrate
	err = db.AutoMigrate(&models.BeautyCenter{}, &models.Personnel{}, &models.PersonnelImage{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Seed BeautyCenter
	bc := models.BeautyCenter{
		Name: "Test Center",
	}
	db.Create(&bc)

	return db
}

func TestPersonnelCRUD(t *testing.T) {
	validate := validator.New()
	db := setupTestDB(t)
	repo := repository.NewPersonnelRepositoryImpl(db)
	svc := service.NewPersonnelServiceImpl(repo, validate)

	var personnelID uint

	t.Run("Find Personnel By ID", func(t *testing.T) {
		res, err := svc.FindByID(personnelID)
		assert.NoError(t, err)
		assert.Equal(t, personnelID, res.ID)
		assert.Len(t, res.Images, 1)
		assert.Equal(t, "http://example.com/img1.jpg", res.Images[0].ImageUrl)
	})

	t.Run("Update Personnel", func(t *testing.T) {
		req := request.UpdatePersonnelRequest{
			Phone: "555555555",
			Images: []request.PersonnelImageRequest{
				{ImageUrl: "http://example.com/img2.jpg", IsCover: true, Position: 1},
			},
		}

		res, err := svc.Update(personnelID, req)
		assert.NoError(t, err)
		assert.Equal(t, "555555555", res.Phone)
		assert.Len(t, res.Images, 1)
		assert.Equal(t, "http://example.com/img2.jpg", res.Images[0].ImageUrl)
	})

	t.Run("Find All Personnel", func(t *testing.T) {
		res, total, err := svc.FindAll(1, 1, 20)
		assert.NoError(t, err)
		assert.NotEmpty(t, res)
		assert.Equal(t, int64(1), total)
		assert.Equal(t, "John", res[0].FirstName)
	})

	t.Run("Delete Personnel (Soft Delete)", func(t *testing.T) {
		err := svc.Delete(personnelID)
		assert.NoError(t, err)

		// Check if it's still in DB but has DeletedAt
		var p models.Personnel
		err = db.Unscoped().First(&p, personnelID).Error
		assert.NoError(t, err)
		assert.NotNil(t, p.DeletedAt)

		// Should not be found by regular FindByID
		_, err = svc.FindByID(personnelID)
		assert.Error(t, err)
	})
}
