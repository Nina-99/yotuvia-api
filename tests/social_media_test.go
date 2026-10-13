package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

func setupSocialMediaTestDB() *gorm.DB {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&models.SocialMedia{})
	return db
}

func TestSocialMediaCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSocialMediaTestDB()
	validate := validator.New()
	repo := repository.NewSocialMediaRepositoryImpl(db)
	svc := service.NewSocialMediaServiceImpl(repo, validate)
	ctrl := controller.NewSocialMediaController(svc)

	r := gin.Default()
	// The controller now scopes writes to the token's centerID claim. These
	// routes bypass the JWT middleware, so inject the actor centre directly —
	// the CRUD contract below is unchanged.
	r.Use(func(ctx *gin.Context) { ctx.Set("centerID", uint(1)) })
	r.POST("/social-media", ctrl.Create)
	r.GET("/social-media/:id", ctrl.FindByID)
	r.PUT("/social-media/:id", ctrl.Update)
	r.DELETE("/social-media/:id", ctrl.Delete)
	r.GET("/beauty-centers/:id/social-media", ctrl.FindByBeautyCenterID)

	t.Run("Create Social Media - Success", func(t *testing.T) {
		reqBody := request.CreateSocialMediaRequest{
			BeautyCenterID: 1,
			Instagram:      "https://instagram.com/test",
			Facebook:       "https://facebook.com/test",
		}
		jsonValue, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/social-media", bytes.NewBuffer(jsonValue))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("Create Social Media - Invalid URL", func(t *testing.T) {
		reqBody := request.CreateSocialMediaRequest{
			BeautyCenterID: 2,
			Instagram:      "not-a-url",
		}
		jsonValue, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/social-media", bytes.NewBuffer(jsonValue))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Current behavior: service validation errors surface as a blanket 500.
		// Aligned to 400 in fix-api-authz-security (MapToHTTP) — out of scope here.
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Get Social Media by Center ID - Success", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/beauty-centers/1/social-media", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, float64(1), response["beauty_center_id"])
	})

	t.Run("Update Social Media - Success", func(t *testing.T) {
		updateBody := request.UpdateSocialMediaRequest{
			Instagram: "https://instagram.com/updated",
		}
		jsonValue, _ := json.Marshal(updateBody)
		req, _ := http.NewRequest("PUT", "/social-media/1", bytes.NewBuffer(jsonValue))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify update
		reqGet, _ := http.NewRequest("GET", "/social-media/1", nil)
		wGet := httptest.NewRecorder()
		r.ServeHTTP(wGet, reqGet)
		var response map[string]interface{}
		json.Unmarshal(wGet.Body.Bytes(), &response)
		assert.Equal(t, "https://instagram.com/updated", response["instagram"])
	})

	t.Run("Delete Social Media - Soft Delete", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/social-media/1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)

		// Verify soft delete (should not be found by normal query)
		reqGet, _ := http.NewRequest("GET", "/social-media/1", nil)
		wGet := httptest.NewRecorder()
		r.ServeHTTP(wGet, reqGet)
		assert.Equal(t, http.StatusNotFound, wGet.Code)

		// Verify it still exists in DB (soft deleted)
		var count int64
		db.Unscoped().Model(&models.SocialMedia{}).Where("id = ?", 1).Count(&count)
		assert.Equal(t, int64(1), count)
	})
}
