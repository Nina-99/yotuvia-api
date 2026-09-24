package tests

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/router"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  C2 — GET /feed-posts/personalized must not N+1
// ──────────────────────────────────────────────

// attachQueryCounter registers a GORM callback that counts every SELECT the
// connection runs (Find, Scan, Count and every preload all flow through the
// query processor). It returns a pointer to the shared counter.
func attachQueryCounter(db *gorm.DB) *int64 {
	var count int64
	db.Callback().Query().After("gorm:query").Register("tests:count_queries", func(*gorm.DB) {
		atomic.AddInt64(&count, 1)
	})
	return &count
}

// measurePersonalizedQueries builds a fresh router over its own SQLite DB,
// seeds `n` active posts each with a watch event and a like for client 5, and
// returns the number of SELECTs the personalized-feed request executes.
func measurePersonalizedQueries(t *testing.T, n int) int64 {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	counter := attachQueryCounter(db)

	r := gin.New()
	router.VideoWatchEventRoutes(r.Group("/api"), db, validator.New())

	seedDashboardCenter(t, db, 1, "Feed Center", "feed@test.com", true)
	rows := make([]models.FeedPost, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, models.FeedPost{
			BeautyCenterID: 1,
			VideoURL:       fmt.Sprintf("/videos/%d.mp4", i),
			ThumbnailURL:   fmt.Sprintf("/thumbs/%d.jpg", i),
			Caption:        fmt.Sprintf("post-%d", i),
			IsActive:       true,
		})
	}
	require.NoError(t, db.Create(&rows).Error)
	for _, p := range rows {
		require.NoError(t, db.Create(&models.VideoWatchEvent{
			ClientID: 5, FeedPostID: p.ID, WatchPercentage: 100, Completed: true,
		}).Error)
		require.NoError(t, db.Create(&models.PostLike{FeedPostID: p.ID, ClientID: 5}).Error)
	}

	atomic.StoreInt64(counter, 0)
	w := do(r, http.MethodGet, "/api/feed-posts/personalized", roleToken(t, 5, 0, "client"), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return atomic.LoadInt64(counter)
}

// TestPersonalizedFeed_QueryCountIsBounded proves the endpoint's SELECT count
// is bounded and does NOT grow with the number of posts. Before the fix the
// request ran one CountByFeedPostID per post plus one FindByID (with 5
// preloads) per watch event and per like; here 30 posts + 30 events + 30 likes
// must stay within a small constant budget.
func TestPersonalizedFeed_QueryCountIsBounded(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)

	small := measurePersonalizedQueries(t, 5)
	large := measurePersonalizedQueries(t, 30)

	t.Logf("personalized feed SELECTs — 5 posts: %d, 30 posts: %d", small, large)

	// Absolute bound: a page-sized request is a handful of statements.
	assert.Lessf(t, large, int64(40),
		"query count must be bounded, got %d SELECTs for 30 posts", large)

	// Non-scaling proof: 6x the posts must not add more than a couple of
	// statements (only the grouped COUNT/IN sizes change, not the query count).
	assert.LessOrEqualf(t, large-small, int64(3),
		"query count must not scale with post count: 5 posts=%d, 30 posts=%d", small, large)
}
