package tests

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
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

	// Seed Personnel
	personnel := models.Personnel{
		BeautyCenterID: bc.ID,
		FirstName:      "John",
		LastName:       "Doe",
		Email:          "john.doe@example.com",
		Phone:          "555123456",
		IsActive:       true,
		PersonnelImage: []models.PersonnelImage{
			{ImageUrl: "http://example.com/img1.jpg", IsCover: true, Position: 1},
		},
	}
	db.Create(&personnel)

	return db
}

func TestPersonnelCRUD(t *testing.T) {
	validate := validator.New()
	db := setupTestDB(t)
	repo := repository.NewPersonnelRepositoryImpl(db)
	svc := service.NewPersonnelServiceImpl(repo, repository.NewServiceRepositoryImpl(db), validate)

	var personnel models.Personnel
	if err := db.First(&personnel).Error; err != nil {
		t.Fatalf("Failed to load seeded personnel: %v", err)
	}
	personnelID := personnel.ID

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
		res, total, err := svc.FindAll(1, nil, 1, 20)
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

// TestUpdateMePersonnelRequestShape guards the self-service allow-list: a staff
// member may edit profile fields (first/last name, phone, password, images)
// but MUST NOT be able to change email, is_active, or beauty_center_id through
// PATCH /personnel/me (design D4).
func TestUpdateMePersonnelRequestShape(t *testing.T) {
	typ := reflect.TypeOf(request.UpdateMePersonnelRequest{})

	allowed := map[string]bool{
		"FirstName": true, "LastName": true, "Phone": true,
		"Password": true, "Images": true,
	}
	blocked := map[string]bool{
		"Email": true, "IsActive": true, "BeautyCenterID": true,
	}

	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		assert.True(t, allowed[name], "field %s must be allowed in UpdateMePersonnelRequest", name)
	}
	for name := range blocked {
		_, found := typ.FieldByName(name)
		assert.False(t, found, "identity/center field %s must NOT exist in UpdateMePersonnelRequest", name)
	}

	// Validator accepts empty (omitempty) values for optional self-edit.
	validate := validator.New()
	empty := request.UpdateMePersonnelRequest{}
	assert.NoError(t, validate.Struct(empty))

	// Password still enforces min length when provided.
	assert.Error(t, validate.Struct(request.UpdateMePersonnelRequest{Password: "short"}))
}

// TestPersonnelService_UpdateMe — the self-service update persists only the
// allow-listed fields, hashes the password, and leaves identity/center fields
// untouched (the DTO structurally cannot carry them).
func TestPersonnelService_UpdateMe(t *testing.T) {
	validate := validator.New()
	db := setupTestDB(t)
	repo := repository.NewPersonnelRepositoryImpl(db)
	svc := service.NewPersonnelServiceImpl(repo, repository.NewServiceRepositoryImpl(db), validate)

	var personnel models.Personnel
	if err := db.First(&personnel).Error; err != nil {
		t.Fatalf("failed to load seeded personnel: %v", err)
	}
	personnelID := personnel.ID

	updated, err := svc.UpdateMe(personnelID, request.UpdateMePersonnelRequest{
		FirstName: "Jane",
		Phone:     "555777888",
		Password:  "new-secret-pass",
		Images: []request.PersonnelImageRequest{
			{ImageUrl: "http://example.com/selfie.jpg", IsCover: true, Position: 1},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "Jane", updated.FirstName)
	assert.Equal(t, "555777888", updated.Phone)
	assert.Len(t, updated.Images, 1)
	assert.Equal(t, "http://example.com/selfie.jpg", updated.Images[0].ImageUrl)

	// Password is hashed in storage, never stored in plaintext.
	var stored models.Personnel
	if err := db.First(&stored, personnelID).Error; err != nil {
		t.Fatalf("failed to reload personnel: %v", err)
	}
	assert.NotEqual(t, "new-secret-pass", stored.Password)
	assert.True(t, utils.VerifyPassword("new-secret-pass", stored.Password), "stored hash must verify")

	// Identity fields are untouched by UpdateMe: still the seeded values.
	assert.Equal(t, "john.doe@example.com", stored.Email)
	assert.Equal(t, uint(1), stored.BeautyCenterID)
	assert.True(t, stored.IsActive)
}

// TestPersonnelUpdateMe_RouteAuthz — HTTP-level coverage for PATCH /personnel/me:
// personnel tokens may self-edit, identity/center fields are rejected 403, a
// client token is denied, and persisted storage only reflects allowed fields.
func TestPersonnelUpdateMe_RouteAuthz(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	r, db := authzTestRouter(t)

	// Personnel with an email identity and an active flag.
	personnel := models.Personnel{
		BeautyCenterID: 1,
		FirstName:      "John",
		LastName:       "Doe",
		Email:          "john.doe@example.com",
		Phone:          "555123456",
		Password:       "old-hash-not-used",
		IsActive:       true,
	}
	if err := db.Create(&personnel).Error; err != nil {
		t.Fatalf("failed to seed personnel: %v", err)
	}

	t.Run("self-update persists allowed fields", func(t *testing.T) {
		w := do(r, http.MethodPatch, "/api/personnel/me", roleToken(t, personnel.ID, 1, "personnel"),
			`{"first_name":"Jane","phone":"555-999","password":"sup3r-secret"}`)
		assert.Equal(t, http.StatusOK, w.Code)

		var stored models.Personnel
		assert.NoError(t, db.First(&stored, personnel.ID).Error)
		assert.Equal(t, "Jane", stored.FirstName)
		assert.Equal(t, "555-999", stored.Phone)
		assert.True(t, utils.VerifyPassword("sup3r-secret", stored.Password), "password persisted hashed")
		assert.Equal(t, true, stored.IsActive, "is_active untouched by self-edit")
		assert.Equal(t, "john.doe@example.com", stored.Email, "email untouched by self-edit")
	})

	t.Run("identity field is structurally impossible (403 contract)", func(t *testing.T) {
		// The DTO has no email field; sending one must still never mutate it.
		w := do(r, http.MethodPatch, "/api/personnel/me", roleToken(t, personnel.ID, 1, "personnel"),
			`{"email":"hacker@evil.com"}`)
		assert.NotEqual(t, http.StatusOK, w.Code, "email is not an allowed self-edit field")

		var stored models.Personnel
		assert.NoError(t, db.First(&stored, personnel.ID).Error)
		assert.Equal(t, "john.doe@example.com", stored.Email, "email unchanged")
	})

	t.Run("client role denied", func(t *testing.T) {
		w := do(r, http.MethodPatch, "/api/personnel/me", roleToken(t, 1, 0, "client"),
			`{"first_name":"Jane"}`)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("center id cannot be reassigned through self-edit", func(t *testing.T) {
		w := do(r, http.MethodPatch, "/api/personnel/me", roleToken(t, personnel.ID, 1, "personnel"),
			`{"beauty_center_id":99}`)
		assert.NotEqual(t, http.StatusOK, w.Code)

		var stored models.Personnel
		assert.NoError(t, db.First(&stored, personnel.ID).Error)
		assert.Equal(t, uint(1), stored.BeautyCenterID, "center unchanged")
	})
}
