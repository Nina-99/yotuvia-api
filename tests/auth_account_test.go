package tests

import (
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthAccountTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// AutoMigrate
	err = db.AutoMigrate(&models.Client{}, &models.AuthAccount{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Seed Client
	client := models.Client{
		FirstName: "Test",
		LastName:  "User",
		Email:     "test@user.com",
	}
	db.Create(&client)

	return db
}

func uintPtr(v uint) *uint {
	return &v
}

func TestAuthAccountRepository(t *testing.T) {
	db := setupAuthAccountTestDB(t)
	repo := repository.NewAuthAccountRepositoryImpl(db)

	var accountID uint

	t.Run("Create AuthAccount Success", func(t *testing.T) {
		account := &models.AuthAccount{
			ClientID:   uintPtr(1),
			Provider:   "google",
			ProviderID: "google-id-123",
		}

		err := repo.Create(account)
		assert.NoError(t, err)
		assert.NotZero(t, account.ID)
		accountID = account.ID
	})

	t.Run("Find AuthAccount By ID with Preload", func(t *testing.T) {
		account, err := repo.FindById(accountID)
		assert.NoError(t, err)
		assert.NotNil(t, account)
		assert.Equal(t, accountID, account.ID)
		assert.Equal(t, uint(1), *account.ClientID)
		assert.NotNil(t, account.Client)
		assert.Equal(t, "test@user.com", account.Client.Email)
	})

	t.Run("Update AuthAccount", func(t *testing.T) {
		account, _ := repo.FindById(accountID)
		account.ProviderID = "new-google-id"

		err := repo.Update(account)
		assert.NoError(t, err)

		updated, _ := repo.FindById(accountID)
		assert.Equal(t, "new-google-id", updated.ProviderID)
	})

	t.Run("Find By Provider", func(t *testing.T) {
		account, err := repo.FindByProvider("google", "new-google-id")
		assert.NoError(t, err)
		assert.NotNil(t, account)
		assert.Equal(t, accountID, account.ID)
	})

	t.Run("Find All AuthAccounts", func(t *testing.T) {
		accounts, total, err := repo.FindAll(1, 20)
		assert.NoError(t, err)
		assert.NotEmpty(t, accounts)
		assert.Equal(t, int64(1), total)
		assert.Equal(t, "test@user.com", accounts[0].Client.Email)
	})

	t.Run("Delete AuthAccount (Soft Delete)", func(t *testing.T) {
		err := repo.Delete(accountID)
		assert.NoError(t, err)

		// Verify soft delete
		var account models.AuthAccount
		err = db.Unscoped().First(&account, accountID).Error
		assert.NoError(t, err)
		assert.NotNil(t, account.DeletedAt)

		// Normal find should fail
		_, err = repo.FindById(accountID)
		assert.Error(t, err)
	})
}
