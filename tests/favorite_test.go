package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupFavoriteTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	err = db.AutoMigrate(&models.Client{}, &models.BeautyCenter{}, &models.Favorite{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	client := models.Client{FirstName: "Fav", LastName: "Client", Email: "fav@client.com", Password: "pwd"}
	db.Create(&client)

	bc := models.BeautyCenter{Name: "Cool Salon", Description: "Nice place", Address: "123 Main", Email: "bc@test.com", Phone: "123", Password: "pwd"}
	db.Create(&bc)

	return db
}

func TestFavoriteCRUD(t *testing.T) {
	db := setupFavoriteTestDB(t)
	repo := repository.NewFavoriteRepositoryImpl(db)
	validate := validator.New()
	svc := service.NewFavoriteServiceImpl(repo, validate)
	ctrl := controller.NewFavoriteController(svc)

	gin.SetMode(gin.TestMode)
	router := gin.Default()

	mockAuthMiddleware := func() gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Set("clientID", uint(1))
			c.Next()
		}
	}

	favorites := router.Group("/favorites").Use(mockAuthMiddleware())
	{
		favorites.POST("", ctrl.Create)
		favorites.DELETE("/:id", ctrl.Delete)
	}
	clients := router.Group("/clients").Use(mockAuthMiddleware())
	{
		clients.GET("/:client_id/favorites", ctrl.FindByClientID)
	}

	t.Run("Create Favorite - Success", func(t *testing.T) {
		reqBody := request.CreateFavoriteRequest{BeautyCenterID: 1}
		jsonValue, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest("POST", "/favorites", bytes.NewBuffer(jsonValue))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), "favorite added successfully")
	})

	t.Run("Create Favorite - Fails if already exists", func(t *testing.T) {
		reqBody := request.CreateFavoriteRequest{BeautyCenterID: 1}
		jsonValue, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest("POST", "/favorites", bytes.NewBuffer(jsonValue))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "already marked as favorite")
	})

	t.Run("Get Favorites By ClientID - Includes Preload", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/clients/1/favorites", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Cool Salon")
	})

	t.Run("Delete Favorite - Success", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/favorites/1", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "favorite removed successfully")
	})

	t.Run("Get Favorites After Delete - Empty", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/clients/1/favorites", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "null", w.Body.String()) // or "null" depending on Go JSON serialization of empty slice
	})
}
