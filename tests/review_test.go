package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/controller"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupReviewTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	err = db.AutoMigrate(&models.Client{}, &models.BeautyCenter{}, &models.Review{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Seed data
	client1 := models.Client{FirstName: "User 1", Email: "u1@test.com"}
	client2 := models.Client{FirstName: "User 2", Email: "u2@test.com"}
	db.Create(&client1)
	db.Create(&client2)

	bc := models.BeautyCenter{Name: "Salon Test"}
	db.Create(&bc)

	review := models.Review{
		ClientID:       client1.ID,
		BeautyCenterID: bc.ID,
		Rating:         5,
		Comment:        "Great!",
	}
	db.Create(&review)

	return db
}

func TestReviewController_CreateValidation(t *testing.T) {
	validate := validator.New()
	gin.SetMode(gin.TestMode)
	db := setupReviewTestDB(t)
	repo := repository.NewReviewRepositoryImpl(db)
	notifSvc := service.NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), repository.NewPushDeviceRepositoryImpl(db))
	svc := service.NewReviewServiceImpl(repo, notifSvc, nil, validate)
	ctrl := controller.NewReviewController(svc)

	r := gin.Default()
	// Mock middleware setting clientID = 1
	r.Use(func(c *gin.Context) {
		c.Set("clientID", uint(1))
		c.Next()
	})
	r.POST("/api/reviews/", ctrl.Create)

	// Test case: Rating > 5 should fail
	reqBody := request.CreateReviewRequest{
		BeautyCenterID: 1,
		Rating:         6, // Invalid
		Comment:        "Too good",
	}
	jsonValue, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/reviews/", bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Current behavior: service validation errors surface as a blanket 500.
	// Aligned to 400 in fix-api-authz-security (MapToHTTP) — out of scope here.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Error:Field validation for 'Rating' failed on the 'max' tag")

	// Test case: Rating < 1 should fail
	reqBody2 := request.CreateReviewRequest{
		BeautyCenterID: 1,
		Rating:         0, // Invalid
		Comment:        "Too bad",
	}
	jsonValue2, _ := json.Marshal(reqBody2)
	req2, _ := http.NewRequest("POST", "/api/reviews/", bytes.NewBuffer(jsonValue2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusInternalServerError, w2.Code)
	assert.Contains(t, w2.Body.String(), "Error:Field validation for 'Rating' failed on the 'required' tag")
}

func TestReviewService_OwnershipUpdateDelete(t *testing.T) {
	validate := validator.New()
	db := setupReviewTestDB(t)
	repo := repository.NewReviewRepositoryImpl(db)
	notifSvc := service.NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), repository.NewPushDeviceRepositoryImpl(db))
	svc := service.NewReviewServiceImpl(repo, notifSvc, nil, validate)

	// User 2 trying to update User 1's review (ID 1). Ownership is a sentinel
	// (apperrors.ErrForbidden), not a free-form string: the controller maps it
	// to 403 {"error":"forbidden"} instead of leaking a 500.
	rating := 4
	updateReq := request.UpdateReviewRequest{Rating: &rating}
	err := svc.Update(2, 1, updateReq) // clientID 2, reviewID 1
	assert.ErrorIs(t, err, apperrors.ErrForbidden)

	// User 2 trying to delete User 1's review
	errDelete := svc.Delete(2, 1) // clientID 2, reviewID 1
	assert.ErrorIs(t, errDelete, apperrors.ErrForbidden)

	// User 1 successfully updating their own review
	errSuccess := svc.Update(1, 1, updateReq)
	assert.NoError(t, errSuccess)
}

func TestReviewRepository_PreloadClient(t *testing.T) {
	db := setupReviewTestDB(t)
	repo := repository.NewReviewRepositoryImpl(db)

	reviews, err := repo.FindByBeautyCenterID(1)
	assert.NoError(t, err)
	assert.Len(t, reviews, 1)
	assert.Equal(t, "User 1", reviews[0].Client.FirstName)      // Proves Client is preloaded
	assert.Equal(t, "Salon Test", reviews[0].BeautyCenter.Name) // Proves BeautyCenter is preloaded
}
