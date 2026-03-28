package tests

import (
	"testing"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/Nina-99/yottu-api/utils"
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
	db := setupTestDB(t)
	repo := repository.NewPersonnelRepositoryImpl(db)
	svc := service.NewPersonnelServiceImpl(repo)

	var personnelID uint

	t.Run("Create Personnel Success", func(t *testing.T) {
		req := request.CreatePersonnelRequest{
			BeautyCenterID: 1,
			FirstName:      "John",
			LastName:       "Doe",
			Email:          "john@example.com",
			Phone:          "123456789",
			Password:       "securepassword",
			IsActive:       true,
			Images: []request.PersonnelImageRequest{
				{ImageUrl: "http://example.com/img1.jpg", IsCover: true, Position: 1},
			},
		}

		res, err := svc.Create(req)
		assert.NoError(t, err)
		assert.Equal(t, "John", res.FirstName)
		assert.Equal(t, "john@example.com", res.Email)
		assert.Len(t, res.Images, 1)
		personnelID = res.ID

		// Verify hashed password in DB
		var p models.Personnel
		db.First(&p, personnelID)
		assert.NotEqual(t, "securepassword", p.Password)
		assert.True(t, utils.VerifyPassword("securepassword", p.Password))
	})

	t.Run("Create Personnel Duplicate Email", func(t *testing.T) {
		req := request.CreatePersonnelRequest{
			BeautyCenterID: 1,
			FirstName:      "Jane",
			LastName:       "Doe",
			Email:          "john@example.com", // Duplicate
			Phone:          "987654321",
			Password:       "password",
		}

		_, err := svc.Create(req)
		assert.Error(t, err)
		assert.Equal(t, "email already in use", err.Error())
	})

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
		res, err := svc.FindAll(1)
		assert.NoError(t, err)
		assert.NotEmpty(t, res)
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
