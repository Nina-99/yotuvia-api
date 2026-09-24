package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/router"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupPersonalizedFeedRouter mounts the PRODUCTION VideoWatchEventRoutes so the
// regression exercises the real composition root instead of a hand-built
// service. The only difference from main is the sqlite in-memory database.
func setupPersonalizedFeedRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	validate := validator.New()

	r := gin.New()
	rg := r.Group("/api")
	router.VideoWatchEventRoutes(rg, db, validate)
	return r, db
}

// TestPersonalizedFeedHTTP_WiredRepositories is the regression for the
// production nil-pointer 500 on GET /api/feed-posts/personalized.
//
// The route is JWT-protected, so the controller always calls
// FeedPostServiceImpl.FindAll with a non-nil clientID, which dereferences
// PostLikeRepository, SavedVideoRepository and CommentRepository. When
// video_watch_event.router.go wired those as nil, the first request panicked.
// This asserts the endpoint answers 200 with a JSON posts array.
func TestPersonalizedFeedHTTP_WiredRepositories(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupPersonalizedFeedRouter(t)

	seedDashboardCenter(t, db, 1, "Feed Center", "feed@test.com", true)
	if err := db.Create(&models.FeedPost{
		BeautyCenterID: 1,
		VideoURL:       "/videos/stream/a.mp4",
		ThumbnailURL:   "/thumbs/a.jpg",
		Caption:        "balayage",
		IsActive:       true,
	}).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}

	w := do(r, http.MethodGet, "/api/feed-posts/personalized", roleToken(t, 5, 0, "client"), "")
	require.Equal(t, http.StatusOK, w.Code, "personalized feed must not 500: %s", w.Body.String())

	var got struct {
		Posts []response.FeedPostResponse `json:"posts"`
		Total int                         `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.NotNil(t, got.Posts, "posts must be a JSON array")
	assert.Len(t, got.Posts, 1)
	assert.Equal(t, 1, got.Total)
	assert.Equal(t, "balayage", got.Posts[0].Caption)
}

// TestPersonalizedFeedHTTP_LikesAndSavesExposed drives the like/save
// repositories with real rows, proving the previously-nil dependencies are now
// wired and reachable end to end: the client's like and save come back as
// is_liked / is_saved on the personalized payload.
func TestPersonalizedFeedHTTP_LikesAndSavesExposed(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupPersonalizedFeedRouter(t)

	seedDashboardCenter(t, db, 1, "Feed Center", "feed@test.com", true)
	post := &models.FeedPost{
		BeautyCenterID: 1,
		VideoURL:       "/videos/stream/b.mp4",
		ThumbnailURL:   "/thumbs/b.jpg",
		Caption:        "corte fade",
		IsActive:       true,
	}
	require.NoError(t, db.Create(post).Error)
	require.NoError(t, db.Create(&models.PostLike{FeedPostID: post.ID, ClientID: 5}).Error)
	require.NoError(t, db.Create(&models.SavedVideo{FeedPostID: post.ID, ClientID: 5}).Error)

	w := do(r, http.MethodGet, "/api/feed-posts/personalized", roleToken(t, 5, 0, "client"), "")
	require.Equal(t, http.StatusOK, w.Code, "personalized feed must not 500: %s", w.Body.String())

	var got struct {
		Posts []response.FeedPostResponse `json:"posts"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Posts, 1)
	assert.True(t, got.Posts[0].IsLiked, "seeded PostLike must surface as is_liked")
	assert.True(t, got.Posts[0].IsSaved, "seeded SavedVideo must surface as is_saved")
}
