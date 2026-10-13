package tests

import (
	"testing"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
//  N2 — watch recording is idempotent per (client, post)
// ──────────────────────────────────────────────

func newWatchService(t *testing.T) (service.VideoWatchEventService, func(clientID, postID uint) []models.VideoWatchEvent) {
	t.Helper()
	db := setupRouterAuthzDB(t)
	watchRepo := repository.NewVideoWatchEventRepositoryImpl(db)
	svc := service.NewVideoWatchEventServiceImpl(
		watchRepo,
		repository.NewFeedPostRepositoryImpl(db),
		repository.NewPostLikeRepositoryImpl(db),
		repository.NewSavedVideoRepositoryImpl(db),
		validator.New(),
	)
	rows := func(clientID, postID uint) []models.VideoWatchEvent {
		var events []models.VideoWatchEvent
		require.NoError(t, db.Where("client_id = ? AND feed_post_id = ?", clientID, postID).Find(&events).Error)
		return events
	}
	return svc, rows
}

// TestRecordWatch_IsIdempotentPerPair — repeated reports for the same
// (client, post) must collapse into ONE row. WatchPercentage/WatchDuration take
// the maximum seen and Completed latches to true and never flips back, so the
// affinity scoring cannot be gamed or downgraded by a later partial report.
func TestRecordWatch_IsIdempotentPerPair(t *testing.T) {
	svc, rows := newWatchService(t)

	require.NoError(t, svc.RecordWatch(5, request.RecordWatchRequest{
		FeedPostID: 1, WatchDuration: 5, WatchPercentage: 30, Completed: false,
	}))
	require.NoError(t, svc.RecordWatch(5, request.RecordWatchRequest{
		FeedPostID: 1, WatchDuration: 8, WatchPercentage: 80, Completed: true,
	}))
	// A later, partial re-report must NOT lower anything.
	require.NoError(t, svc.RecordWatch(5, request.RecordWatchRequest{
		FeedPostID: 1, WatchDuration: 2, WatchPercentage: 10, Completed: false,
	}))
	// A second post is a distinct pair and gets its own row.
	require.NoError(t, svc.RecordWatch(5, request.RecordWatchRequest{
		FeedPostID: 2, WatchPercentage: 5,
	}))

	got := rows(5, 1)
	require.Len(t, got, 1, "three reports for the same pair must yield a single row")
	assert.Equal(t, 80.0, got[0].WatchPercentage, "percentage must keep the maximum")
	assert.Equal(t, 8, got[0].WatchDuration, "duration must keep the maximum")
	assert.True(t, got[0].Completed, "completed must latch to true and never flip back")

	other := rows(5, 2)
	require.Len(t, other, 1, "a different post is a distinct pair")
}
