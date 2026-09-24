package tests

import (
	"encoding/json"
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
//  Shop the look — the feed carries the tagged service's offer
//  (service_price / service_duration), sourced from the already-preloaded
//  post.Service. No new query is issued; absent association => absent key.
// ──────────────────────────────────────────────

// setupFeedServiceOfferRouter mounts both feed surfaces: FeedPostRoutes owns
// the public list / by-center / mine endpoints, VideoWatchEventRoutes owns the
// personalized feed.
func setupFeedServiceOfferRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	validate := validator.New()

	r := gin.New()
	rg := r.Group("/api")
	router.FeedPostRoutes(rg, db, validate)
	router.VideoWatchEventRoutes(rg, db, validate)
	return r, db
}

// seedOfferService inserts a service with a chosen price and duration (the
// shared seedDashboardService hardcodes 50/30, which cannot express a free
// service).
func seedOfferService(t *testing.T, db *gorm.DB, id, centerID uint, name string, price float64, duration int) *models.Service {
	t.Helper()
	svc := &models.Service{
		ID:             id,
		BeautyCenterID: centerID,
		CategoryID:     1,
		Name:           name,
		Description:    "desc " + name,
		Duration:       duration,
		Price:          price,
		IsActive:       true,
	}
	require.NoError(t, db.Create(svc).Error)
	return svc
}

// firstRawPost decodes a {"posts":[...]} payload and returns the first post as
// raw key->value so a test can assert a key is physically ABSENT, not zero.
func firstRawPost(t *testing.T, w []byte) map[string]json.RawMessage {
	t.Helper()
	var payload struct {
		Posts []map[string]json.RawMessage `json:"posts"`
	}
	require.NoError(t, json.Unmarshal(w, &payload))
	require.Len(t, payload.Posts, 1)
	return payload.Posts[0]
}

func decodeFloat(t *testing.T, raw map[string]json.RawMessage, key string) float64 {
	t.Helper()
	var v float64
	require.NoError(t, json.Unmarshal(raw[key], &v))
	return v
}

func decodeInt(t *testing.T, raw map[string]json.RawMessage, key string) int {
	t.Helper()
	var v int
	require.NoError(t, json.Unmarshal(raw[key], &v))
	return v
}

// TestFeedServiceOffer_TaggedPostExposesPriceAndDuration — a video tagged with
// a service must report that service's real price and duration.
func TestFeedServiceOffer_TaggedPostExposesPriceAndDuration(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupFeedServiceOfferRouter(t)

	seedDashboardCenter(t, db, 1, "Offer Center", "offer@test.com", true)
	svc := seedOfferService(t, db, 1, 1, "Balayage", 149.9, 45)
	require.NoError(t, db.Create(&models.FeedPost{
		BeautyCenterID: 1,
		ServiceID:      &svc.ID,
		VideoURL:       "/videos/offer-a.mp4",
		ThumbnailURL:   "/thumbs/a.jpg",
		Caption:        "balayage",
		IsActive:       true,
	}).Error)

	w := do(r, http.MethodGet, "/api/feed-posts", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	raw := firstRawPost(t, w.Body.Bytes())

	require.Contains(t, raw, "service_price")
	require.Contains(t, raw, "service_duration")
	assert.Equal(t, 149.9, decodeFloat(t, raw, "service_price"))
	assert.Equal(t, 45, decodeInt(t, raw, "service_duration"))
}

// TestFeedServiceOffer_UntaggedPostOmitsBothKeys — a video with no tagged
// service omits both keys entirely (nil pointers + omitempty), so the client
// can distinguish "no offer" from a zero-valued one.
func TestFeedServiceOffer_UntaggedPostOmitsBothKeys(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupFeedServiceOfferRouter(t)

	seedDashboardCenter(t, db, 1, "Offer Center", "offer@test.com", true)
	require.NoError(t, db.Create(&models.FeedPost{
		BeautyCenterID: 1,
		VideoURL:       "/videos/offer-b.mp4",
		ThumbnailURL:   "/thumbs/b.jpg",
		Caption:        "no service",
		IsActive:       true,
	}).Error)

	w := do(r, http.MethodGet, "/api/feed-posts", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	raw := firstRawPost(t, w.Body.Bytes())

	assert.NotContains(t, raw, "service_price")
	assert.NotContains(t, raw, "service_duration")
	assert.NotContains(t, raw, "service_id")
}

// TestFeedServiceOffer_FreeServiceReportsZeroNotAbsent — a tagged service
// priced 0 (free) must emit service_price: 0, NOT omit the key. This is the
// whole reason the fields are pointers.
func TestFeedServiceOffer_FreeServiceReportsZeroNotAbsent(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupFeedServiceOfferRouter(t)

	seedDashboardCenter(t, db, 1, "Offer Center", "offer@test.com", true)
	svc := seedOfferService(t, db, 1, 1, "Free Trim", 0, 20)
	require.NoError(t, db.Create(&models.FeedPost{
		BeautyCenterID: 1,
		ServiceID:      &svc.ID,
		VideoURL:       "/videos/offer-c.mp4",
		ThumbnailURL:   "/thumbs/c.jpg",
		Caption:        "free trim",
		IsActive:       true,
	}).Error)

	w := do(r, http.MethodGet, "/api/feed-posts", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	raw := firstRawPost(t, w.Body.Bytes())

	require.Contains(t, raw, "service_price")
	require.Contains(t, raw, "service_duration")
	assert.Equal(t, 0.0, decodeFloat(t, raw, "service_price"))
	assert.Equal(t, 20, decodeInt(t, raw, "service_duration"))
}

// TestFeedServiceOffer_AllReadSurfacesCarryTheOffer — every surface that builds
// a FeedPostResponse (public feed, personalized feed, by-center, mine) routes
// through mapToResponse and must expose the same offer keys for a tagged post.
func TestFeedServiceOffer_AllReadSurfacesCarryTheOffer(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupFeedServiceOfferRouter(t)

	seedDashboardCenter(t, db, 1, "Offer Center", "offer@test.com", true)
	svc := seedOfferService(t, db, 1, 1, "Color", 80, 60)
	require.NoError(t, db.Create(&models.FeedPost{
		BeautyCenterID: 1,
		ServiceID:      &svc.ID,
		VideoURL:       "/videos/offer-d.mp4",
		ThumbnailURL:   "/thumbs/d.jpg",
		Caption:        "color",
		IsActive:       true,
	}).Error)

	assertOffer := func(t *testing.T, raw map[string]json.RawMessage, surface string) {
		t.Helper()
		require.Contains(t, raw, "service_price", surface)
		require.Contains(t, raw, "service_duration", surface)
		assert.Equal(t, 80.0, decodeFloat(t, raw, "service_price"), surface)
		assert.Equal(t, 60, decodeInt(t, raw, "service_duration"), surface)
	}

	// Public feed.
	w := do(r, http.MethodGet, "/api/feed-posts", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assertOffer(t, firstRawPost(t, w.Body.Bytes()), "public feed")

	// Personalized feed (JWT-protected).
	w = do(r, http.MethodGet, "/api/feed-posts/personalized", roleToken(t, 5, 0, "client"), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assertOffer(t, firstRawPost(t, w.Body.Bytes()), "personalized feed")

	// By-center (bare array payload).
	w = do(r, http.MethodGet, "/api/feed-posts/center/1", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var centerArr []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &centerArr))
	require.Len(t, centerArr, 1)
	assertOffer(t, centerArr[0], "by-center")

	// Mine (center management surface).
	w = do(r, http.MethodGet, "/api/feed-posts/mine", roleToken(t, 1, 1, "beauty_center"), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assertOffer(t, firstRawPost(t, w.Body.Bytes()), "mine")
}

// measureFeedOfferQueries builds a fresh router over its own SQLite DB, seeds
// `n` active tagged posts and returns the number of SELECTs one public feed
// request runs. Surfacing the offer must reuse the existing Service preload.
func measureFeedOfferQueries(t *testing.T, n int) int64 {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	counter := attachQueryCounter(db)

	r := gin.New()
	router.FeedPostRoutes(r.Group("/api"), db, validator.New())

	seedDashboardCenter(t, db, 1, "Offer Center", "offer@test.com", true)
	svc := seedOfferService(t, db, 1, 1, "Cut", 50, 30)
	rows := make([]models.FeedPost, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, models.FeedPost{
			BeautyCenterID: 1,
			ServiceID:      &svc.ID,
			VideoURL:       fmt.Sprintf("/videos/q-%d.mp4", i),
			ThumbnailURL:   fmt.Sprintf("/thumbs/q-%d.jpg", i),
			Caption:        fmt.Sprintf("post-%d", i),
			IsActive:       true,
		})
	}
	require.NoError(t, db.Create(&rows).Error)

	atomic.StoreInt64(counter, 0)
	w := do(r, http.MethodGet, "/api/feed-posts?limit=100", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return atomic.LoadInt64(counter)
}

// TestFeedServiceOffer_QueryCountIsBounded proves surfacing the offer adds no
// per-row query: the SELECT count for a public feed page is small and does not
// grow with the number of tagged posts (Service was already preloaded).
func TestFeedServiceOffer_QueryCountIsBounded(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)

	small := measureFeedOfferQueries(t, 3)
	large := measureFeedOfferQueries(t, 25)

	t.Logf("public feed SELECTs — 3 tagged posts: %d, 25 tagged posts: %d", small, large)

	assert.Lessf(t, large, int64(40),
		"query count must be bounded, got %d SELECTs for 25 posts", large)
	assert.LessOrEqualf(t, large-small, int64(3),
		"query count must not scale with post count: 3 posts=%d, 25 posts=%d", small, large)
}
