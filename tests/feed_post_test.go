package tests

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// HTTP-level smoke for the NEW feed contract (feat-video-feed-unification):
// multipart create, author-scoped writes, suspend endpoint, public list.
// Identity comes from injected claims (mirrors JWTAuthMiddleware).

func setupFeedHTTPRouter(t *testing.T, db *gorm.DB, role string, userID, centerID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	zapLogger := zap.NewNop()

	feedRepo := repository.NewFeedPostRepositoryImpl(db)
	bcRepo := repository.NewBeautyCenterRepositoryImpl(db)
	personnelRepo := repository.NewPersonnelRepositoryImpl(db)
	serviceRepo := repository.NewServiceRepositoryImpl(db)
	svc := service.NewFeedPostServiceImpl(feedRepo, bcRepo, personnelRepo, serviceRepo, nil, nil, nil, nil)
	ctrl := controller.NewFeedPostController(svc)

	r := gin.New()
	// Fake auth middleware injecting claims like JWTAuthMiddleware would.
	r.Use(func(c *gin.Context) {
		c.Set("role", role)
		c.Set("clientID", userID)
		if centerID != 0 {
			c.Set("centerID", centerID)
		}
		c.Next()
	})

	posts := r.Group("/feed-posts")
	{
		posts.GET("", ctrl.FindAll)
		posts.POST("", middleware.RequireRole("personnel", "beauty_center"), ctrl.Create)
		posts.PUT("/:id", ctrl.Update)
		posts.DELETE("/:id", ctrl.Delete)
		posts.PUT("/:id/suspend", middleware.RequireRole("beauty_center"), ctrl.Suspend)
	}
	_ = zapLogger
	return r
}

func multipartUpload(t *testing.T, caption string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("caption", caption)
	part, err := writer.CreateFormFile("video", "clip.mp4")
	assert.NoError(t, err)
	_, err = part.Write([]byte("fake-video"))
	assert.NoError(t, err)
	assert.NoError(t, writer.Close())
	contentType := writer.FormDataContentType()
	return body, contentType
}

// setupFeedPostTestDB mirrors the shared authz migration set (includes
// feed_posts and its associations).
func setupFeedPostTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return setupRouterAuthzDB(t)
}

func TestFeedHTTP_NewContract(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupFeedPostTestDB(t)

	seedDashboardCenter(t, db, 1, "HTTP Center", "http@test.com", true)
	seedDashboardPersonnel(t, db, 7, 1, true)

	// Center creates a post via multipart.
	centerRouter := setupFeedHTTPRouter(t, db, "beauty_center", 1, 1)
	body, contentType := multipartUpload(t, "Promo de apertura")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/feed-posts", body)
	req.Header.Set("Content-Type", contentType)
	centerRouter.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Public list renders it.
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/feed-posts", nil)
	centerRouter.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Body.String(), "Promo de apertura")

	// Staff cannot delete a center-authored post.
	staffRouter := setupFeedHTTPRouter(t, db, "personnel", 7, 0)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("DELETE", "/feed-posts/1", nil)
	staffRouter.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusForbidden, w3.Code)

	// Author deletes it.
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("DELETE", "/feed-posts/1", nil)
	centerRouter.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusOK, w4.Code)

	// Client role cannot create posts at all.
	clientRouter := setupFeedHTTPRouter(t, db, "client", 5, 0)
	body5, ct5 := multipartUpload(t, "intruso")
	w5 := httptest.NewRecorder()
	req5, _ := http.NewRequest("POST", "/feed-posts", body5)
	req5.Header.Set("Content-Type", ct5)
	clientRouter.ServeHTTP(w5, req5)
	assert.Equal(t, http.StatusForbidden, w5.Code)
}

// TestFeedHTTP_SuspendRoundTrip covers center moderation over staff videos.
func TestFeedHTTP_SuspendRoundTrip(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupFeedPostTestDB(t)
	seedDashboardCenter(t, db, 1, "Suspend Center", "sus@test.com", true)
	seedDashboardPersonnel(t, db, 9, 1, true)

	// Staff uploads directly through the repository-backed service.
	pID := uint(9)
	post := &models.FeedPost{
		BeautyCenterID: 1,
		PersonnelID:    &pID,
		VideoURL:       "/videos/stream/staff.mp4",
		Caption:        "Mi trabajo",
		IsActive:       true,
	}
	assert.NoError(t, db.Create(post).Error)

	centerRouter := setupFeedHTTPRouter(t, db, "beauty_center", 1, 1)

	// Suspend with note.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/feed-posts/1/suspend", bytes.NewBufferString(`{"note":"Fuera de política"}`))
	req.Header.Set("Content-Type", "application/json")
	centerRouter.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var after models.FeedPost
	assert.NoError(t, db.First(&after, 1).Error)
	assert.False(t, after.IsActive)
	assert.Equal(t, "Fuera de política", after.ModerationNote)

	// Re-enable with empty note.
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("PUT", "/feed-posts/1/suspend", bytes.NewBufferString(`{}`))
	req2.Header.Set("Content-Type", "application/json")
	centerRouter.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	var restored models.FeedPost
	assert.NoError(t, db.First(&restored, 1).Error)
	assert.True(t, restored.IsActive)
	assert.Empty(t, restored.ModerationNote)
}

var _ = gorm.ErrRecordNotFound // reserved
