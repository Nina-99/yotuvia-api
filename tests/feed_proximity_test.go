package tests

// Location in the feed — the behaviour a client actually feels.
//
// Rule: a post from a centre the viewer could visit outranks a far more popular
// post from another city, and a promoted campaign from another city never spends
// the advertiser's budget on that viewer.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// feedNearSantaCruz seeds two centres ~550 km apart and one post each:
//   - local:  in Santa Cruz, no traction at all
//   - remote: in La Paz, 1000 views (a far more "engaging" post)
//
// Everything else (recency, promo flag, affinity) is identical, so the only
// difference the ranking can react to is distance and engagement.
func feedNearSantaCruz(t *testing.T) (*gin.Engine, *gorm.DB, uint, uint) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := setupRouterAuthzDB(t)
	// The shared authz schema does not carry the ad tables.
	require.NoError(t, db.AutoMigrate(&models.BoostCampaign{}))

	r := gin.New()
	router.VideoWatchEventRoutes(r.Group("/api"), db, validator.New())

	local := models.BeautyCenter{
		Name: "Santa Cruz", Email: "scz@test.com", ReferralCode: "SCZ",
		Latitude: -17.7833, Longitude: -63.1821, IsActive: true,
	}
	remote := models.BeautyCenter{
		Name: "La Paz", Email: "lpz@test.com", ReferralCode: "LPZ",
		Latitude: -16.5000, Longitude: -68.1500, IsActive: true,
	}
	require.NoError(t, db.Create(&local).Error)
	require.NoError(t, db.Create(&remote).Error)

	localPost := models.FeedPost{
		BeautyCenterID: local.ID, VideoURL: "/v/local.mp4", ThumbnailURL: "/t/local.jpg",
		Caption: "local", IsActive: true, ViewsCount: 0,
	}
	remotePost := models.FeedPost{
		BeautyCenterID: remote.ID, VideoURL: "/v/remote.mp4", ThumbnailURL: "/t/remote.jpg",
		Caption: "remote", IsActive: true, ViewsCount: 1000,
	}
	require.NoError(t, db.Create(&localPost).Error)
	require.NoError(t, db.Create(&remotePost).Error)

	return r, db, localPost.ID, remotePost.ID
}

// orderedIDs extracts the post ids in the order the API returned them.
func orderedIDs(t *testing.T, body []byte) []uint {
	t.Helper()
	var payload struct {
		Posts []struct {
			ID uint `json:"id"`
		} `json:"posts"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))
	ids := make([]uint, 0, len(payload.Posts))
	for _, p := range payload.Posts {
		ids = append(ids, p.ID)
	}
	return ids
}

func TestFeed_LocalContentOutranksPopularContentElsewhere(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, _, localID, remoteID := feedNearSantaCruz(t)
	token := roleToken(t, 5, 0, "client")

	t.Run("without a position, raw engagement wins", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/feed-posts/personalized?limit=50", token, "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		ids := orderedIDs(t, w.Body.Bytes())
		require.Len(t, ids, 2)
		assert.Equal(t, remoteID, ids[0],
			"1000 views must beat zero views when the server knows nothing about where the client is")
	})

	t.Run("with a position, content from another region is not even a candidate", func(t *testing.T) {
		// The viewer stands in the same plaza as the local centre. The 550 km post
		// never enters the pool: the ranking no longer wastes work ordering content
		// nobody in this city could act on.
		w := do(r, http.MethodGet,
			"/api/feed-posts/personalized?limit=50&lat=-17.7833&lng=-63.1821", token, "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		ids := orderedIDs(t, w.Body.Bytes())
		require.Len(t, ids, 1)
		assert.Equal(t, localID, ids[0],
			"a post 550 km away must not be served to a viewer who cannot visit it")
	})

	t.Run("a malformed position degrades to no position", func(t *testing.T) {
		w := do(r, http.MethodGet,
			"/api/feed-posts/personalized?limit=50&lat=abc&lng=-63.18", token, "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		ids := orderedIDs(t, w.Body.Bytes())
		require.Len(t, ids, 2)
		assert.Equal(t, remoteID, ids[0], "garbage coordinates must not rank anything")
	})
}

// TestBoostAuction_RespectsTheViewerRadius — the advertiser's budget must not
// be spent on viewers who cannot reach the centre.
func TestBoostAuction_RespectsTheViewerRadius(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	t.Setenv("BOOST_MAX_RADIUS_KM", "50")
	r, db, _, remoteID := feedNearSantaCruz(t)

	// A campaign for the LA PAZ post, well funded and inside its window.
	now := time.Now()
	require.NoError(t, db.Create(&models.BoostCampaign{
		BeautyCenterID: 2, FeedPostID: remoteID,
		BudgetMilli: 100_000, BidCPMMilli: 5_000,
		Status: "active", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour),
	}).Error)

	// Enough organic posts in BOTH cities that each viewer's page has ad slots
	// (one per 5 positions). With the geo prefilter, fillers from the other city
	// are not candidates, so the pool must be local to the viewer.
	for _, centerID := range []uint{1, 2} {
		for i := 0; i < 10; i++ {
			require.NoError(t, db.Create(&models.FeedPost{
				BeautyCenterID: centerID,
				VideoURL:       fmt.Sprintf("/v/fill-%d-%d.mp4", centerID, i),
				ThumbnailURL:   fmt.Sprintf("/t/fill-%d-%d.jpg", centerID, i),
				Caption:        fmt.Sprintf("fill-%d-%d", centerID, i),
				IsActive:       true,
			}).Error)
		}
	}

	token := roleToken(t, 5, 0, "client")

	t.Run("a viewer in Santa Cruz is never served the La Paz ad", func(t *testing.T) {
		w := do(r, http.MethodGet,
			"/api/feed-posts/personalized?limit=50&lat=-17.7833&lng=-63.1821", token, "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		var payload struct {
			Posts []struct {
				ID        uint `json:"id"`
				IsBoosted bool `json:"is_boosted"`
			} `json:"posts"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
		for _, p := range payload.Posts {
			assert.False(t, p.IsBoosted,
				"a promoted slot 550 km away must not be served (post %d)", p.ID)
		}
	})

	t.Run("a viewer in La Paz does receive it", func(t *testing.T) {
		w := do(r, http.MethodGet,
			"/api/feed-posts/personalized?limit=50&lat=-16.5000&lng=-68.1500", token, "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		var payload struct {
			Posts []struct {
				ID        uint `json:"id"`
				IsBoosted bool `json:"is_boosted"`
			} `json:"posts"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))

		boosted := false
		for _, p := range payload.Posts {
			if p.IsBoosted {
				boosted = true
			}
		}
		assert.True(t, boosted, "the campaign must serve viewers in its own city")
	})
}

// TestPublicFeed_PromoFlagDoesNotOutrankRecency — `is_promo` is a LABEL, not a
// ranking signal. Ordering by it gave any centre free top-of-feed placement,
// which undercut the paid boost: the platform's only sellable placement.
func TestPublicFeed_PromoFlagDoesNotOutrankRecency(t *testing.T) {
	db := setupRouterAuthzDB(t)

	older := models.FeedPost{
		BeautyCenterID: 1, VideoURL: "/v/older.mp4", ThumbnailURL: "/t/older.jpg",
		Caption: "older promo", IsActive: true, IsPromo: true,
	}
	newer := models.FeedPost{
		BeautyCenterID: 1, VideoURL: "/v/newer.mp4", ThumbnailURL: "/t/newer.jpg",
		Caption: "newer", IsActive: true, IsPromo: false,
	}
	require.NoError(t, db.Create(&older).Error)
	require.NoError(t, db.Create(&newer).Error)

	// CreatedAt lives on the embedded gorm.Model and cannot be set in a literal;
	// age the promo post explicitly so recency is what decides the order.
	require.NoError(t, db.Model(&models.FeedPost{}).Where("id = ?", older.ID).
		UpdateColumn("created_at", time.Now().Add(-48*time.Hour)).Error)

	posts, _, err := repository.NewFeedPostRepositoryImpl(db).FindAllActive(1, 20)
	require.NoError(t, err)
	require.Len(t, posts, 2)

	assert.Equal(t, newer.ID, posts[0].ID,
		"the newest post must lead: the free promo flag no longer buys placement")
	assert.Equal(t, older.ID, posts[1].ID)
}

// TestBoostAuction_SkipsAVideoTheViewerAlreadyWatched — serve-level frequency
// control. Re-serving a consumed ad annoys the viewer and buys the advertiser no
// extra reach; the signal (a watch event) already exists.
func TestBoostAuction_SkipsAVideoTheViewerAlreadyWatched(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db, _, remoteID := feedNearSantaCruz(t)

	now := time.Now()
	require.NoError(t, db.Create(&models.BoostCampaign{
		BeautyCenterID: 2, FeedPostID: remoteID,
		BudgetMilli: 100_000, BidCPMMilli: 5_000,
		Status: "active", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour),
	}).Error)
	for _, centerID := range []uint{1, 2} {
		for i := 0; i < 10; i++ {
			require.NoError(t, db.Create(&models.FeedPost{
				BeautyCenterID: centerID,
				VideoURL:       fmt.Sprintf("/v/fill-%d-%d.mp4", centerID, i),
				ThumbnailURL:   fmt.Sprintf("/t/fill-%d-%d.jpg", centerID, i),
				Caption:        fmt.Sprintf("fill-%d-%d", centerID, i),
				IsActive:       true,
			}).Error)
		}
	}
	// Client 5 already watched the promoted video.
	require.NoError(t, db.Create(&models.VideoWatchEvent{
		ClientID: 5, FeedPostID: remoteID, WatchPercentage: 100, Completed: true,
	}).Error)

	boostedFor := func(clientID uint) bool {
		w := do(r, http.MethodGet,
			"/api/feed-posts/personalized?limit=50&lat=-16.5000&lng=-68.1500",
			roleToken(t, clientID, 0, "client"), "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		var payload struct {
			Posts []struct {
				ID        uint `json:"id"`
				IsBoosted bool `json:"is_boosted"`
			} `json:"posts"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
		for _, p := range payload.Posts {
			if p.ID == remoteID && p.IsBoosted {
				return true
			}
		}
		return false
	}

	assert.False(t, boostedFor(5), "a video the viewer already watched must not be promoted to them again")
	assert.True(t, boostedFor(6), "a fresh viewer must still receive the campaign")
}
