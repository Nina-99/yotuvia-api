package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  Video → booking funnel: attribution + book taps
// ──────────────────────────────────────────────

// reservationAttributionLite is the subset of ReservationResponse the
// attribution scenarios assert on.
type reservationAttributionLite struct {
	ID           uint  `json:"id"`
	SourcePostID *uint `json:"source_post_id"`
}

func seedAttributionPost(t *testing.T, db *gorm.DB, centerID uint, caption string) *models.FeedPost {
	t.Helper()
	post := &models.FeedPost{
		BeautyCenterID: centerID,
		VideoURL:       "/uploads/videos/" + caption + ".mp4",
		ThumbnailURL:   "/uploads/thumbs/" + caption + ".jpg",
		Caption:        caption,
		IsActive:       true,
	}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("failed to seed feed post: %v", err)
	}
	return post
}

// TestReservationAttribution_ClientBooking covers the frozen contract for the
// optional source_post_id on the client booking request:
//
//   - a valid id resolves, is stored, and is echoed on the ReservationResponse;
//   - absent, zero and non-existent ids are IGNORED — the booking still lands
//     with a nil attribution (a tracking field must never fail a real booking).
func TestReservationAttribution_ClientBooking(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reservationWalkinTestRouter(t)
	seedWalkinData(t, db)

	post := seedAttributionPost(t, db, 1, "promo-corte")
	clientToken := roleToken(t, 1, 0, "client")

	tests := []struct {
		name         string
		sourceJSON   string
		startTime    string
		wantResolved bool
	}{
		{
			name:         "valid post id is stored and echoed",
			sourceJSON:   fmt.Sprintf(`"source_post_id":%d`, post.ID),
			startTime:    "11:00",
			wantResolved: true,
		},
		{
			name:         "absent post id books with nil attribution",
			sourceJSON:   `"client_id":0`,
			startTime:    "12:00",
			wantResolved: false,
		},
		{
			name:         "zero post id books with nil attribution",
			sourceJSON:   `"source_post_id":0`,
			startTime:    "13:00",
			wantResolved: false,
		},
		{
			name:         "non-existent post id books with nil attribution",
			sourceJSON:   `"source_post_id":999999`,
			startTime:    "14:00",
			wantResolved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := fmt.Sprintf(
				`{"beauty_center_id":1,"personnel_id":1,"service_ids":[1],"date":"2026-08-16","start_time":%q,%s}`,
				tt.startTime, tt.sourceJSON,
			)
			w := do(r, http.MethodPost, "/api/reservation/", clientToken, body)
			require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

			var stored models.Reservation
			require.NoError(t, db.Where(
				"client_id = ? AND start_time = ?", 1, tt.startTime,
			).First(&stored).Error, "booking must be persisted")

			if tt.wantResolved {
				require.NotNil(t, stored.SourcePostID, "valid id must be persisted")
				assert.Equal(t, post.ID, *stored.SourcePostID)
			} else {
				assert.Nil(t, stored.SourcePostID, "unresolved tracking must be dropped, not fail the booking")
			}

			// The ReservationResponse DTO must carry the attribution when set.
			w2 := do(r, http.MethodGet, fmt.Sprintf("/api/reservation/%d", stored.ID), clientToken, "")
			require.Equal(t, http.StatusOK, w2.Code, "body: %s", w2.Body.String())
			var resp reservationAttributionLite
			require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
			assert.Equal(t, stored.ID, resp.ID)
			if tt.wantResolved {
				require.NotNil(t, resp.SourcePostID)
				assert.Equal(t, post.ID, *resp.SourcePostID)
			} else {
				assert.Nil(t, resp.SourcePostID)
			}
		})
	}
}

// TestReservationAttribution_AdminWalkinUnaffected proves the admin/walk-in
// creation path keeps working and simply leaves source_post_id nil — the
// client-only hint never leaks into it.
func TestReservationAttribution_AdminWalkinUnaffected(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reservationWalkinTestRouter(t)
	seedWalkinData(t, db)

	// A post exists, but the admin body has no field to reference it.
	seedAttributionPost(t, db, 1, "promo-admin")

	adminToken := roleToken(t, 7, 1, "beauty_center")
	body := `{"beauty_center_id":1,"client_id":4,"personnel_id":1,"service_ids":[1],"date":"2026-08-16","start_time":"15:00","source_post_id":1}`
	w := do(r, http.MethodPost, "/api/reservation/admin", adminToken, body)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var stored models.Reservation
	require.NoError(t, db.Where(
		"client_id = ? AND start_time = ?", 4, "15:00",
	).First(&stored).Error)
	assert.Nil(t, stored.SourcePostID, "admin walk-ins never carry attribution")
}

// TestFeedBookTaps_IncrementAndExpose covers the new public counter endpoint:
// exactly one increment per call and book_taps surfaced on the feed response.
func TestFeedBookTaps_IncrementAndExpose(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupFeedPostTestDB(t)
	seedDashboardCenter(t, db, 1, "Taps Center", "taps@test.com", true)

	post := seedAttributionPost(t, db, 1, "taps-clip")
	r := setupFeedHTTPRouter(t, db, "client", 5, 0)

	tap := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/feed-posts/%d/book-taps", post.ID), nil)
		r.ServeHTTP(w, req)
		return w
	}

	w1 := tap()
	assert.Equal(t, http.StatusOK, w1.Code, "body: %s", w1.Body.String())

	var afterOne models.FeedPost
	require.NoError(t, db.First(&afterOne, post.ID).Error)
	assert.Equal(t, 1, afterOne.BookTaps, "one call = exactly one increment")

	w2 := tap()
	assert.Equal(t, http.StatusOK, w2.Code)

	var afterTwo models.FeedPost
	require.NoError(t, db.First(&afterTwo, post.ID).Error)
	assert.Equal(t, 2, afterTwo.BookTaps, "two calls = two increments")

	// The public feed response must expose the counter.
	wf := httptest.NewRecorder()
	reqf, _ := http.NewRequest(http.MethodGet, "/feed-posts", nil)
	r.ServeHTTP(wf, reqf)
	assert.Equal(t, http.StatusOK, wf.Code)
	assert.Contains(t, wf.Body.String(), `"book_taps":2`)
}

// TestFeedBookTaps_ErrorCases covers the endpoint's edge behaviour. It mirrors
// the existing views endpoint: an unknown post is a silent, best-effort no-op
// (200, writes nothing), while a malformed id is a 400.
func TestFeedBookTaps_ErrorCases(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupFeedPostTestDB(t)
	seedDashboardCenter(t, db, 1, "Taps Center", "taps@test.com", true)
	r := setupFeedHTTPRouter(t, db, "client", 5, 0)

	t.Run("unknown post is a 200 no-op", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/feed-posts/999999/book-taps", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "book tap counted")

		var count int64
		require.NoError(t, db.Model(&models.FeedPost{}).Where("id = ?", 999999).Count(&count).Error)
		assert.Zero(t, count, "an unknown post must never be created as a side effect")
	})

	t.Run("malformed id is a 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/feed-posts/abc/book-taps", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
