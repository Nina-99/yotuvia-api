package tests

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

// ──────────────────────────────────────────────
//  N5 — user-facing list queries are bounded server-side
// ──────────────────────────────────────────────

// boundedCreatedAt returns an increasing timestamp so "newest survive" can be
// asserted deterministically.
func boundedCreatedAt(i int) time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute)
}

func TestPostLikeRepo_FindByPostIDIsBounded(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewPostLikeRepositoryImpl(db)

	n := repository.ListQueryLimit + 5
	for i := 0; i < n; i++ {
		require.NoError(t, db.Create(&models.PostLike{
			Model:      gorm.Model{CreatedAt: boundedCreatedAt(i)},
			FeedPostID: 1,
			ClientID:   uint(i + 1),
		}).Error)
	}

	got, err := repo.FindByPostID(1)
	require.NoError(t, err)
	assert.Len(t, got, repository.ListQueryLimit, "public like list must be capped")
	assert.Equal(t, uint(n), got[0].ClientID, "newest like must survive the cap")
}

func TestPostLikeRepo_FindByClientIDIsBounded(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewPostLikeRepositoryImpl(db)

	n := repository.ListQueryLimit + 5
	for i := 0; i < n; i++ {
		require.NoError(t, db.Create(&models.PostLike{
			Model:      gorm.Model{CreatedAt: boundedCreatedAt(i)},
			FeedPostID: uint(i + 1),
			ClientID:   5,
		}).Error)
	}

	got, err := repo.FindByClientID(5)
	require.NoError(t, err)
	assert.Len(t, got, repository.ListQueryLimit)
	assert.Equal(t, uint(n), got[0].FeedPostID, "newest like must survive the cap")
}

func TestSavedVideoRepo_FindByClientIDIsBounded(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewSavedVideoRepositoryImpl(db)

	n := repository.ListQueryLimit + 5
	for i := 0; i < n; i++ {
		require.NoError(t, db.Create(&models.SavedVideo{
			Model:      gorm.Model{CreatedAt: boundedCreatedAt(i)},
			FeedPostID: uint(i + 1),
			ClientID:   5,
		}).Error)
	}

	got, err := repo.FindByClientID(5)
	require.NoError(t, err)
	assert.Len(t, got, repository.ListQueryLimit)
	assert.Equal(t, uint(n), got[0].FeedPostID, "newest save must survive the cap")
}

func TestVideoWatchEventRepo_FindByClientIDIsBounded(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewVideoWatchEventRepositoryImpl(db)

	n := repository.ListQueryLimit + 5
	for i := 0; i < n; i++ {
		require.NoError(t, db.Create(&models.VideoWatchEvent{
			Model:      gorm.Model{CreatedAt: boundedCreatedAt(i)},
			ClientID:   5,
			FeedPostID: uint(i + 1),
		}).Error)
	}

	got, err := repo.FindByClientID(5)
	require.NoError(t, err)
	assert.Len(t, got, repository.ListQueryLimit)
	assert.Equal(t, uint(n), got[0].FeedPostID, "newest watch event must survive the cap")
}

func TestFeedPostRepo_ManagementListsAreBounded(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewFeedPostRepositoryImpl(db)

	n := repository.ListQueryLimit + 5
	personnelID := uint(7)
	created := make([]*models.FeedPost, 0, n)
	for i := 0; i < n; i++ {
		p := &models.FeedPost{
			Model:          gorm.Model{CreatedAt: boundedCreatedAt(i)},
			BeautyCenterID: 1,
			PersonnelID:    &personnelID,
			VideoURL:       fmt.Sprintf("/videos/%d.mp4", i),
			ThumbnailURL:   fmt.Sprintf("/thumbs/%d.jpg", i),
			Caption:        fmt.Sprintf("p-%03d", i),
			IsActive:       true,
		}
		require.NoError(t, db.Create(p).Error)
		created = append(created, p)
	}
	newest := created[n-1]

	t.Run("FindActiveByCenter", func(t *testing.T) {
		got, err := repo.FindActiveByCenter(1)
		require.NoError(t, err)
		assert.Len(t, got, repository.ListQueryLimit)
		assert.Equal(t, newest.ID, got[0].ID, "newest post must survive the cap")
	})

	t.Run("FindAllByCenter", func(t *testing.T) {
		got, err := repo.FindAllByCenter(1)
		require.NoError(t, err)
		assert.Len(t, got, repository.ListQueryLimit)
		assert.Equal(t, newest.ID, got[0].ID)
	})

	t.Run("FindByPersonnelID", func(t *testing.T) {
		got, err := repo.FindByPersonnelID(7)
		require.NoError(t, err)
		assert.Len(t, got, repository.ListQueryLimit)
		assert.Equal(t, newest.ID, got[0].ID)
	})
}

// TestFeedPostRepo_InternalPathsStayUncapped proves the cap is scoped to
// user-facing lists: the personalized feed's internal scan (FindAllActive up
// to 1000) and the ID-scoped batch helper (FindByIDs) must keep working above
// ListQueryLimit.
func TestFeedPostRepo_InternalPathsStayUncapped(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewFeedPostRepositoryImpl(db)

	n := repository.ListQueryLimit + 5
	ids := make([]uint, 0, n)
	for i := 0; i < n; i++ {
		p := &models.FeedPost{
			BeautyCenterID: 1,
			VideoURL:       fmt.Sprintf("/videos/i-%d.mp4", i),
			ThumbnailURL:   fmt.Sprintf("/thumbs/i-%d.jpg", i),
			Caption:        fmt.Sprintf("i-%03d", i),
			IsActive:       true,
		}
		require.NoError(t, db.Create(p).Error)
		ids = append(ids, p.ID)
	}

	active, _, err := repo.FindAllActive(1, 1000)
	require.NoError(t, err)
	assert.Len(t, active, n, "internal scan must not be capped at ListQueryLimit")

	batch, err := repo.FindByIDs(ids)
	require.NoError(t, err)
	assert.Len(t, batch, n, "ID-scoped batch helper must not be capped")
}

// TestPublicFeedLimitIsClamped — GET /feed-posts?limit=... clamps the page size
// to maxFeedLimit (100) and echoes the effective value back, so a client cannot
// drag the entire feed through the preload pipeline.
func TestPublicFeedLimitIsClamped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	r := gin.New()
	router.FeedPostRoutes(r.Group("/api"), db, validator.New())

	seedDashboardCenter(t, db, 1, "Feed Center", "feed@test.com", true)
	for i := 0; i < 5; i++ {
		seedActiveFeedPost(t, db, 1, fmt.Sprintf("clamp-%d", i))
	}

	w := do(r, http.MethodGet, "/api/feed-posts?limit=100000", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var got struct {
		Limit int `json:"limit"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, 100, got.Limit, "limit must be clamped to the server-side cap")
}
