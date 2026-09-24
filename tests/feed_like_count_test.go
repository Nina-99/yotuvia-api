package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/router"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
//  C3 — the feed's likes_count must reflect real post_likes
// ──────────────────────────────────────────────

// TestFeed_LikesCountReflectsRealRows drives the real toggle endpoint and
// asserts the public feed reports the live COUNT(post_likes): the stored
// FeedPost.LikesCount column is seeder-only fiction and must not leak.
func TestFeed_LikesCountReflectsRealRows(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)

	r := gin.New()
	api := r.Group("/api")
	router.FeedPostRoutes(api, db, validator.New())
	router.PostLikeRoutes(api, db, validator.New())

	seedDashboardCenter(t, db, 1, "Like Center", "like@test.com", true)
	post := &models.FeedPost{
		BeautyCenterID: 1,
		VideoURL:       "/videos/like.mp4",
		ThumbnailURL:   "/thumbs/like.jpg",
		Caption:        "corte",
		IsActive:       true,
		LikesCount:     999, // stale/seeded fiction — must be ignored
	}
	require.NoError(t, db.Create(post).Error)

	readLikesCount := func() int {
		w := do(r, http.MethodGet, "/api/feed-posts", "", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var got struct {
			Posts []response.FeedPostResponse `json:"posts"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
		require.Len(t, got.Posts, 1)
		return got.Posts[0].LikesCount
	}
	liveLikes := func() int64 {
		var n int64
		require.NoError(t, db.Model(&models.PostLike{}).Where("feed_post_id = ?", post.ID).Count(&n).Error)
		return n
	}

	assert.Equal(t, 0, readLikesCount(), "stored LikesCount=999 must not leak; no live likes yet")

	token := roleToken(t, 5, 0, "client")
	body := fmt.Sprintf(`{"feed_post_id":%d}`, post.ID)

	// Like on.
	w := do(r, http.MethodPost, "/api/post-likes/toggle", token, body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, int64(1), liveLikes())
	assert.Equal(t, 1, readLikesCount(), "likes_count must equal the real row count")
	assert.Equal(t, 1, readLikesCount(), "repeat read stays correct")

	// Like off (soft-delete).
	w = do(r, http.MethodPost, "/api/post-likes/toggle", token, body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, int64(0), liveLikes())
	assert.Equal(t, 0, readLikesCount(), "unlike must drop likes_count back to 0")
}
