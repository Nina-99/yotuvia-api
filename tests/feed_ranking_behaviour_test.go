package tests

// Behavioural specs for the ranking rewrite (block 1-5): the feed must not
// collapse into one category, a skip must repel, quality must beat raw volume,
// and "already watched" must mean ALL of it — not the last 200.

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

type rankingFixture struct {
	engine *gin.Engine
	db     *gorm.DB
	// mk creates an active post whose Service belongs to categoryID.
	mk func(categoryID uint, views, taps int) models.FeedPost
}

func newRankingFixture(t *testing.T) rankingFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	r := gin.New()
	router.VideoWatchEventRoutes(r.Group("/api"), db, validator.New())

	center := models.BeautyCenter{
		Name: "C", Email: "rank@test.com", ReferralCode: "RANK", IsActive: true,
		Latitude: -17.78, Longitude: -63.18,
	}
	require.NoError(t, db.Create(&center).Error)

	// One Service row per category, created on demand: the ranker reads the
	// category from post.Service.CategoryID (FeedPost has no category column).
	services := map[uint]uint{}
	serviceFor := func(categoryID uint) uint {
		if id, ok := services[categoryID]; ok {
			return id
		}
		svc := models.Service{
			BeautyCenterID: center.ID, CategoryID: categoryID,
			Name: fmt.Sprintf("svc-%d", categoryID), Description: "d",
			Duration: 30, Price: 100, IsActive: true,
		}
		require.NoError(t, db.Create(&svc).Error)
		services[categoryID] = svc.ID
		return svc.ID
	}

	seq := 0
	mk := func(categoryID uint, views, taps int) models.FeedPost {
		seq++
		serviceID := serviceFor(categoryID)
		post := models.FeedPost{
			BeautyCenterID: center.ID,
			ServiceID:      &serviceID,
			VideoURL:       fmt.Sprintf("/v/%d.mp4", seq),
			ThumbnailURL:   fmt.Sprintf("/t/%d.jpg", seq),
			Caption:        fmt.Sprintf("post-%d", seq),
			ViewsCount:     views,
			BookTaps:       taps,
			IsActive:       true,
		}
		require.NoError(t, db.Create(&post).Error)
		return post
	}

	return rankingFixture{engine: r, db: db, mk: mk}
}

func (f rankingFixture) watch(t *testing.T, postID uint, pct float64, completed bool) {
	t.Helper()
	require.NoError(t, f.db.Create(&models.VideoWatchEvent{
		ClientID: 5, FeedPostID: postID, WatchPercentage: pct, Completed: completed,
	}).Error)
}

func (f rankingFixture) personalizedIDs(t *testing.T) []uint {
	t.Helper()
	ids, _ := f.page(t, "limit=50", "")
	return ids
}

// page performs one ranked request and returns the ids plus the reported total.
func (f rankingFixture) page(t *testing.T, query, session string) ([]uint, int) {
	t.Helper()
	url := "/api/feed-posts/personalized?" + query
	if session != "" {
		url += "&session=" + session
	}
	w := do(f.engine, http.MethodGet, url, roleToken(t, 5, 0, "client"), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var payload struct {
		Posts []struct {
			ID uint `json:"id"`
		} `json:"posts"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	ids := make([]uint, 0, len(payload.Posts))
	for _, p := range payload.Posts {
		ids = append(ids, p.ID)
	}
	return ids, payload.Total
}

// TestRanking_SkipsRepelTheCategory — a video closed almost immediately is
// evidence AGAINST its category. Before this change it added +1, i.e. the ranker
// was taught to show more of whatever the user rejected.
func TestRanking_SkipsRepelTheCategory(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	f := newRankingFixture(t)

	// Three skipped videos in the rejected category.
	rejectedCat := uint(41)
	for i := 0; i < 3; i++ {
		p := f.mk(rejectedCat, 0, 0)
		f.watch(t, p.ID, 1, false) // 1% watched = a skip
	}

	inRejected := f.mk(rejectedCat, 0, 0)
	inNeutral := f.mk(42, 0, 0)

	ids := f.personalizedIDs(t)
	require.Len(t, ids, 2, "watched posts are excluded; both candidates remain")
	assert.Equal(t, inNeutral.ID, ids[0],
		"a category the client keeps abandoning must rank below a neutral one")
	assert.Equal(t, inRejected.ID, ids[1])
}

// TestRanking_QualityBeatsVolume — 9 taps of 10 views must outrank 10 taps of
// 10,000: views measure exposure (the slide was shown), not interest.
func TestRanking_QualityBeatsVolume(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	f := newRankingFixture(t)

	highQuality := f.mk(50, 10, 9)
	bigAndBoring := f.mk(50, 10_000, 10)

	ids := f.personalizedIDs(t)
	require.Len(t, ids, 2)
	assert.Equal(t, highQuality.ID, ids[0],
		"a 90%% tap rate must beat a 0.1%% tap rate with far more absolute views")
	assert.Equal(t, bigAndBoring.ID, ids[1])
}

// TestRanking_WatchedExclusionIsComplete — the signal window is capped at 200
// events, but the EXCLUSION must not be: a heavy viewer used to start seeing the
// oldest videos again.
func TestRanking_WatchedExclusionIsComplete(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	f := newRankingFixture(t)

	var first models.FeedPost
	for i := 0; i < 205; i++ {
		p := f.mk(60, 0, 0)
		if i == 0 {
			first = p
		}
		f.watch(t, p.ID, 100, true)
	}
	unseen := f.mk(60, 0, 0)

	ids := f.personalizedIDs(t)

	assert.Contains(t, ids, unseen.ID)
	assert.NotContains(t, ids, first.ID,
		"the oldest watched post must not return just because it left the 200-event window")
	assert.Len(t, ids, 1, "every other candidate was already watched")
}

// TestRanking_SaturatedAffinityDoesNotCollapseTheFeed — the anti-collapse rule.
// Twenty completed watches in one category must not bury a nearby, high-quality
// post from another category.
func TestRanking_SaturatedAffinityDoesNotCollapseTheFeed(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	f := newRankingFixture(t)

	obsession := uint(70)
	for i := 0; i < 20; i++ {
		p := f.mk(obsession, 0, 0)
		f.watch(t, p.ID, 100, true) // twenty completed watches in one category
	}

	// The obsessed category, but a terrible post: no taps, old.
	weak := f.mk(obsession, 0, 0)
	require.NoError(t, f.db.Model(&models.FeedPost{}).Where("id = ?", weak.ID).
		UpdateColumns(map[string]interface{}{
			"views_count": 1000, "book_taps": 0,
			"created_at": time.Now().Add(-60 * 24 * time.Hour),
		}).Error)

	// Another category, excellent quality, brand new, same centre (so proximity
	// and center affinity are identical for both).
	strong := f.mk(71, 100, 60)

	ids := f.personalizedIDs(t)
	require.Len(t, ids, 2)
	assert.Equal(t, strong.ID, ids[0],
		"a saturated affinity must not outrank a fresh 60%%-tap post: affinity is a signal, not a verdict")
}

// TestRanking_PaginationIsACursorOverOneRanking — page 2 must continue the same
// ordering. It used to jump to the generic newest-first feed, which duplicated
// page 1 and abandoned the ranking after ten items.
func TestRanking_PaginationIsACursorOverOneRanking(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	f := newRankingFixture(t)

	for i := 0; i < 15; i++ {
		f.mk(80, 0, 0)
	}

	page1, total1 := f.page(t, "limit=10&offset=0", "sess-A")
	page2, total2 := f.page(t, "limit=10&offset=10", "sess-A")

	require.Len(t, page1, 10)
	require.Len(t, page2, 5)
	assert.Equal(t, 15, total1)
	assert.Equal(t, total1, total2, "the total is the ranking size, not the page size")

	seen := map[uint]bool{}
	for _, id := range append(append([]uint{}, page1...), page2...) {
		assert.False(t, seen[id], "post %d appears on both pages", id)
		seen[id] = true
	}
	assert.Len(t, seen, 15, "the two pages together cover the whole ranking exactly once")
}

// TestRanking_SessionSeedIsStable — the same session gets the same order twice;
// a different session is free to differ (that is the point of the seed).
func TestRanking_SessionSeedIsStable(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	f := newRankingFixture(t)

	for i := 0; i < 12; i++ {
		f.mk(81, i*10, i) // varied scores so ties are not carrying the assertion
	}

	first, _ := f.page(t, "limit=10&offset=0", "sess-stable")
	second, _ := f.page(t, "limit=10&offset=0", "sess-stable")
	assert.Equal(t, first, second, "one session must see one stable ranking")

	other, _ := f.page(t, "limit=10&offset=0", "sess-other")
	assert.Len(t, other, 10, "a different session still gets a full page")
}

// TestRanking_ImpressionsAreRecordedAndMeasured — the instrumentation that makes
// any future ranking change falsifiable.
func TestRanking_ImpressionsAreRecordedAndMeasured(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	f := newRankingFixture(t)

	for i := 0; i < 5; i++ {
		f.mk(82, 0, 0)
	}

	// Serving page 1 logs the positions.
	_, _ = f.page(t, "limit=5&offset=0", "sess-imp")

	var logged int64
	require.NoError(t, f.db.Model(&models.FeedImpression{}).Count(&logged).Error)
	assert.Equal(t, int64(5), logged, "the first page is logged, position 1-based")

	// A watch on the post served at position 1 makes that position engaged.
	var first models.FeedImpression
	require.NoError(t, f.db.Order("position ASC").First(&first).Error)
	assert.Equal(t, 1, first.Position)
	f.watch(t, first.PostID, 100, true)

	repo := repository.NewFeedImpressionRepositoryImpl(f.db)
	metrics, err := repo.PositionMetrics(time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.Len(t, metrics, 5)

	// One served post per position: position 1 was served once and engaged once.
	assert.Equal(t, int64(1), metrics[0].Engaged, "the watched post counts as engaged")
	assert.Equal(t, int64(1), metrics[0].Impressions)
	// The repository returns raw counts; the service derives the rate.
}

// TestRanking_ImpressionMetricsIgnoreViewsFromBeforeTheServe — an old view must
// not be credited to a later impression.
func TestRanking_ImpressionMetricsIgnoreViewsFromBeforeTheServe(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	f := newRankingFixture(t)

	post := f.mk(83, 0, 0)
	// A watch BEFORE the impression.
	f.watch(t, post.ID, 100, true)
	require.NoError(t, f.db.Model(&models.VideoWatchEvent{}).
		Where("feed_post_id = ?", post.ID).
		UpdateColumn("created_at", time.Now().Add(-2*time.Hour)).Error)

	// The impression happens now.
	require.NoError(t, f.db.Create(&models.FeedImpression{
		PostID: post.ID, ClientID: 5, Position: 1, SessionID: "sess-old",
	}).Error)

	repo := repository.NewFeedImpressionRepositoryImpl(f.db)
	metrics, err := repo.PositionMetrics(time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	assert.Zero(t, metrics[0].Engaged,
		"a view that happened before the impression proves nothing about it")
}
