package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/router"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  GET /feed-posts/personnel/:id — one professional's public videos
// ──────────────────────────────────────────────

func setupPersonnelFeedRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	r := gin.New()
	router.FeedPostRoutes(r.Group("/api"), db, validator.New())
	return r, db
}

// TestPersonnelFeed_OnlyActivePostsOfThatPersonnelNewestFirst — the endpoint
// returns just the professional's ACTIVE videos, newest first, in the same
// {"posts":[...],"total":n} shape as the public feed.
func TestPersonnelFeed_OnlyActivePostsOfThatPersonnelNewestFirst(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupPersonnelFeedRouter(t)

	seedDashboardCenter(t, db, 1, "Feed Center", "feed@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	svc := seedOfferService(t, db, 1, 1, "Balayage", 149.9, 45)
	seedClient(t, db, 5, "viewer", "viewer@test.com")

	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	postA := &models.FeedPost{
		BeautyCenterID: 1, PersonnelID: uintPtr(1), ServiceID: &svc.ID,
		VideoURL: "/videos/a.mp4", ThumbnailURL: "/thumbs/a.jpg", Caption: "a",
		IsActive: true, Model: gorm.Model{CreatedAt: older},
	}
	postB := &models.FeedPost{
		BeautyCenterID: 1, PersonnelID: uintPtr(1),
		VideoURL: "/videos/b.mp4", ThumbnailURL: "/thumbs/b.jpg", Caption: "b",
		IsActive: true, Model: gorm.Model{CreatedAt: newer},
	}
	require.NoError(t, db.Create(postA).Error)
	require.NoError(t, db.Create(postB).Error)
	// Inactive post of the same professional — must be excluded. NOTE: the
	// model's `default:true` tag would drop an inserted zero value, so the row
	// is deactivated explicitly after insert.
	inactive := &models.FeedPost{
		BeautyCenterID: 1, PersonnelID: uintPtr(1),
		VideoURL: "/videos/c.mp4", ThumbnailURL: "/thumbs/c.jpg", Caption: "inactive",
	}
	require.NoError(t, db.Create(inactive).Error)
	require.NoError(t, db.Model(inactive).Update("is_active", false).Error)
	// Another professional's post — must be excluded.
	require.NoError(t, db.Create(&models.FeedPost{
		BeautyCenterID: 1, PersonnelID: uintPtr(2),
		VideoURL: "/videos/d.mp4", ThumbnailURL: "/thumbs/d.jpg", Caption: "other",
		IsActive: true,
	}).Error)

	clientID := uint(5)
	require.NoError(t, db.Create(&models.PostLike{FeedPostID: postA.ID, ClientID: clientID}).Error)
	require.NoError(t, db.Create(&models.SavedVideo{FeedPostID: postA.ID, ClientID: clientID}).Error)
	require.NoError(t, db.Create(&models.VideoComment{FeedPostID: postA.ID, ClientID: &clientID, Body: "bien"}).Error)

	w := do(r, http.MethodGet, "/api/feed-posts/personnel/1", roleToken(t, clientID, 0, "client"), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var got struct {
		Posts []response.FeedPostResponse `json:"posts"`
		Total int                         `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Posts, 2, "only the professional's two active posts")
	assert.Equal(t, 2, got.Total)
	assert.Equal(t, postB.ID, got.Posts[0].ID, "newest post first")
	assert.Equal(t, postA.ID, got.Posts[1].ID)

	tagged := got.Posts[1]
	require.NotNil(t, tagged.ServicePrice)
	assert.Equal(t, 149.9, *tagged.ServicePrice)
	require.NotNil(t, tagged.ServiceDuration)
	assert.Equal(t, 45, *tagged.ServiceDuration)
	assert.Equal(t, 1, tagged.LikesCount)
	assert.Equal(t, 1, tagged.CommentsCount)
	assert.True(t, tagged.IsLiked)
	assert.True(t, tagged.IsSaved)

	untagged := got.Posts[0]
	assert.Nil(t, untagged.ServicePrice, "untagged post must not carry an offer")
	assert.True(t, untagged.IsActive)
}

// TestPersonnelFeed_PublicWithoutToken — the endpoint serves anonymous
// visitors; the response is still the same shape.
func TestPersonnelFeed_PublicWithoutToken(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupPersonnelFeedRouter(t)

	seedDashboardCenter(t, db, 1, "Feed Center", "feed@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	require.NoError(t, db.Create(&models.FeedPost{
		BeautyCenterID: 1, PersonnelID: uintPtr(1),
		VideoURL: "/videos/pub.mp4", ThumbnailURL: "/thumbs/pub.jpg", Caption: "pub",
		IsActive: true,
	}).Error)

	w := do(r, http.MethodGet, "/api/feed-posts/personnel/1", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var got struct {
		Posts []response.FeedPostResponse `json:"posts"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Posts, 1)
	assert.False(t, got.Posts[0].IsLiked, "anonymous viewer has no like state")
}

// measurePersonnelFeedQueries seeds `n` active posts for professional 1 and
// returns the number of SELECTs one GET /feed-posts/personnel/1 runs.
func measurePersonnelFeedQueries(t *testing.T, n int) int64 {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	counter := attachQueryCounter(db)

	r := gin.New()
	router.FeedPostRoutes(r.Group("/api"), db, validator.New())

	seedDashboardCenter(t, db, 1, "Feed Center", "feed@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	rows := make([]models.FeedPost, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, models.FeedPost{
			BeautyCenterID: 1,
			PersonnelID:    uintPtr(1),
			VideoURL:       fmt.Sprintf("/videos/p-%d.mp4", i),
			ThumbnailURL:   fmt.Sprintf("/thumbs/p-%d.jpg", i),
			Caption:        fmt.Sprintf("post-%d", i),
			IsActive:       true,
		})
	}
	require.NoError(t, db.Create(&rows).Error)
	for _, p := range rows {
		require.NoError(t, db.Create(&models.PostLike{FeedPostID: p.ID, ClientID: 5}).Error)
		require.NoError(t, db.Create(&models.VideoComment{FeedPostID: p.ID, ClientID: uintPtr(5), Body: "x"}).Error)
	}

	atomic.StoreInt64(counter, 0)
	w := do(r, http.MethodGet, "/api/feed-posts/personnel/1", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return atomic.LoadInt64(counter)
}

// TestPersonnelFeed_QueryCountIsBounded proves the endpoint's SELECT count is
// bounded and does NOT grow with the number of posts: it reuses the shared
// preloads and the grouped like/comment COUNTs.
func TestPersonnelFeed_QueryCountIsBounded(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)

	small := measurePersonnelFeedQueries(t, 3)
	large := measurePersonnelFeedQueries(t, 25)

	t.Logf("personnel feed SELECTs — 3 posts: %d, 25 posts: %d", small, large)

	assert.Lessf(t, large, int64(40),
		"query count must be bounded, got %d SELECTs for 25 posts", large)
	assert.LessOrEqualf(t, large-small, int64(3),
		"query count must not scale with post count: 3 posts=%d, 25 posts=%d", small, large)
}
