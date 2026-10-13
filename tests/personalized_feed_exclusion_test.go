package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  N4 — the personalized feed excludes watched posts, but never goes empty
// ──────────────────────────────────────────────

func seedActiveFeedPost(t *testing.T, db *gorm.DB, centerID uint, caption string) *models.FeedPost {
	t.Helper()
	p := &models.FeedPost{
		BeautyCenterID: centerID,
		VideoURL:       fmt.Sprintf("/videos/%s.mp4", caption),
		ThumbnailURL:   fmt.Sprintf("/thumbs/%s.jpg", caption),
		Caption:        caption,
		IsActive:       true,
	}
	require.NoError(t, db.Create(p).Error)
	return p
}

func personalizedPostIDs(t *testing.T, body []byte) []uint {
	t.Helper()
	var got struct {
		Posts []response.FeedPostResponse `json:"posts"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	ids := make([]uint, 0, len(got.Posts))
	for _, p := range got.Posts {
		ids = append(ids, p.ID)
	}
	return ids
}

// TestPersonalizedFeed_ExcludesWatchedButStaysNonEmpty proves the exclusion
// list is actually wired: previously the only caller passed nil, so watched
// content re-appeared in the personalized feed. The client watched posts[0]
// and posts[1]; only the unwatched posts[2] must come back.
func TestPersonalizedFeed_ExcludesWatchedButStaysNonEmpty(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupPersonalizedFeedRouter(t)
	seedDashboardCenter(t, db, 1, "Feed Center", "feed@test.com", true)

	posts := []*models.FeedPost{
		seedActiveFeedPost(t, db, 1, "watched-a"),
		seedActiveFeedPost(t, db, 1, "watched-b"),
		seedActiveFeedPost(t, db, 1, "unwatched"),
	}
	for _, p := range posts[:2] {
		require.NoError(t, db.Create(&models.VideoWatchEvent{
			ClientID: 5, FeedPostID: p.ID, WatchPercentage: 100, Completed: true,
		}).Error)
	}

	w := do(r, http.MethodGet, "/api/feed-posts/personalized", roleToken(t, 5, 0, "client"), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	ids := personalizedPostIDs(t, w.Body.Bytes())
	assert.NotContains(t, ids, posts[0].ID, "watched post must be excluded")
	assert.NotContains(t, ids, posts[1].ID, "watched post must be excluded")
	assert.Contains(t, ids, posts[2].ID, "unwatched post must be present")
	assert.NotEmpty(t, ids, "feed must not be empty while unwatched content exists")
}

// TestPersonalizedFeed_FallsBackWhenEverythingWatched proves the non-empty
// fallback: when a heavy client has watched every active post, exclusion would
// empty the feed, so the endpoint falls back to the unexcluded ranking instead
// of returning nothing.
func TestPersonalizedFeed_FallsBackWhenEverythingWatched(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupPersonalizedFeedRouter(t)
	seedDashboardCenter(t, db, 1, "Feed Center", "feed@test.com", true)

	posts := []*models.FeedPost{
		seedActiveFeedPost(t, db, 1, "all-a"),
		seedActiveFeedPost(t, db, 1, "all-b"),
	}
	for _, p := range posts {
		require.NoError(t, db.Create(&models.VideoWatchEvent{
			ClientID: 5, FeedPostID: p.ID, WatchPercentage: 100, Completed: true,
		}).Error)
	}

	w := do(r, http.MethodGet, "/api/feed-posts/personalized", roleToken(t, 5, 0, "client"), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	ids := personalizedPostIDs(t, w.Body.Bytes())
	assert.NotEmpty(t, ids, "feed must fall back rather than go empty when everything is watched")
	assert.Len(t, ids, len(posts))
}
