package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPostLikeTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	err = db.AutoMigrate(
		&models.BeautyCenter{},
		&models.FeedPost{},
		&models.Client{},
		&models.PostLike{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Seed data
	db.Create(&models.BeautyCenter{Name: "Test BC"})
	db.Create(&models.FeedPost{BeautyCenterID: 1, VideoURL: "test.mp4", ThumbnailURL: "test.jpg", Caption: "Test Post"})
	db.Create(&models.Client{FirstName: "Test", LastName: "User", Email: "test@user.com"})

	return db
}

// MockMiddleware sets a clientID in context for testing protected routes
func MockMiddleware(clientID uint) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("clientID", clientID)
		c.Next()
	}
}

func TestPostLikeCRUD(t *testing.T) {
	validate := validator.New()
	gin.SetMode(gin.TestMode)
	db := setupPostLikeTestDB(t)

	repo := repository.NewPostLikeRepositoryImpl(db)
	svc := service.NewPostLikeServiceImpl(repo, validate)
	ctrl := controller.NewPostLikeController(svc)

	r := gin.Default()
	api := r.Group("/api")

	// We use the real router setup but override the middleware to avoid JWT validation
	// Actually, let's just define the routes here to be more precise for the test
	likes := api.Group("/post-likes")
	{
		likes.GET("/post/:post_id", ctrl.GetLikesByPost)

		// Protected routes with Mock Middleware
		protected := likes.Group("")
		protected.Use(MockMiddleware(1)) // Assume clientID 1
		{
			protected.POST("", ctrl.Create)
			protected.DELETE("/post/:post_id", ctrl.Delete)
			protected.GET("/post/:post_id/check", ctrl.CheckIfUserLiked)
		}
	}

	t.Run("POST /api/post-likes - Success", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"feed_post_id": 1,
		}
		jsonValue, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/api/post-likes", bytes.NewBuffer(jsonValue))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), `"FeedPostID":1`)
		assert.Contains(t, w.Body.String(), `"ClientID":1`)
	})

	t.Run("POST /api/post-likes - Duplicate Prevention", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"feed_post_id": 1,
		}
		jsonValue, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/api/post-likes", bytes.NewBuffer(jsonValue))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "already liked this post")
	})

	t.Run("GET /api/post-likes/post/:post_id/check - True", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/post-likes/post/1/check", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"liked":true`)
	})

	t.Run("GET /api/post-likes/post/:post_id - List", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/post-likes/post/1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"client_id":1`)
	})

	t.Run("DELETE /api/post-likes/post/:post_id - Success", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/post-likes/post/1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Like removed")

		// Verify deletion
		var count int64
		db.Model(&models.PostLike{}).Where("feed_post_id = ? AND client_id = ?", 1, 1).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("GET /api/post-likes/post/:post_id/check - False", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/post-likes/post/1/check", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"liked":false`)
	})

	t.Run("DELETE /api/post-likes/post/:post_id - Not Found", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/post-likes/post/1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "like not found")
	})
}
