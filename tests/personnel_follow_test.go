package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
//  Follow a professional — POST/DELETE /personnel/:id/follow
//  plus is_following on GET /personnel/:id
// ──────────────────────────────────────────────

// TestPersonnelFollow_FollowIsIdempotent — a double POST keeps exactly one row
// and answers {"following":true} both times (the DB unique index guarantees it).
func TestPersonnelFollow_FollowIsIdempotent(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := authzTestRouter(t)

	seedDashboardCenter(t, db, 1, "Follow Center", "follow@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedClient(t, db, 5, "follower", "follower@test.com")

	token := roleToken(t, 5, 0, "client")

	for i := 0; i < 2; i++ {
		w := do(r, http.MethodPost, "/api/personnel/1/follow", token, "")
		require.Equal(t, http.StatusCreated, w.Code, "attempt %d: %s", i+1, w.Body.String())
		var body struct {
			Following bool `json:"following"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.True(t, body.Following, "attempt %d must report following=true", i+1)
	}

	var count int64
	require.NoError(t, db.Model(&models.PersonnelFollow{}).
		Where("client_id = ? AND personnel_id = ?", 5, 1).Count(&count).Error)
	assert.Equal(t, int64(1), count, "double follow must not duplicate rows")
}

// TestPersonnelFollow_UnfollowIsIdempotent — DELETE of a followed edge then of
// a missing edge both answer 200 {"following":false} and leave zero rows.
func TestPersonnelFollow_UnfollowIsIdempotent(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := authzTestRouter(t)

	seedDashboardCenter(t, db, 1, "Follow Center", "follow@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedClient(t, db, 5, "follower", "follower@test.com")

	token := roleToken(t, 5, 0, "client")
	require.Equal(t, http.StatusCreated, do(r, http.MethodPost, "/api/personnel/1/follow", token, "").Code)

	for i := 0; i < 2; i++ {
		w := do(r, http.MethodDelete, "/api/personnel/1/follow", token, "")
		require.Equal(t, http.StatusOK, w.Code, "attempt %d: %s", i+1, w.Body.String())
		var body struct {
			Following bool `json:"following"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.False(t, body.Following, "attempt %d must report following=false", i+1)
	}

	var count int64
	require.NoError(t, db.Model(&models.PersonnelFollow{}).
		Where("client_id = ? AND personnel_id = ?", 5, 1).Count(&count).Error)
	assert.Zero(t, count, "unfollow must leave no rows")
}

// TestPersonnelFollow_IsFollowingReflectsState — GET /personnel/:id carries
// is_following derived from the caller's JWT claim: false before the follow,
// true after.
func TestPersonnelFollow_IsFollowingReflectsState(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := authzTestRouter(t)

	seedDashboardCenter(t, db, 1, "Follow Center", "follow@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedClient(t, db, 5, "follower", "follower@test.com")

	token := roleToken(t, 5, 0, "client")

	readFollowing := func() bool {
		w := do(r, http.MethodGet, "/api/personnel/1", token, "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var got response.PersonnelResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
		return got.IsFollowing
	}

	assert.False(t, readFollowing(), "is_following must be false before following")
	require.Equal(t, http.StatusCreated, do(r, http.MethodPost, "/api/personnel/1/follow", token, "").Code)
	assert.True(t, readFollowing(), "is_following must be true after following")
	require.Equal(t, http.StatusOK, do(r, http.MethodDelete, "/api/personnel/1/follow", token, "").Code)
	assert.False(t, readFollowing(), "is_following must return to false after unfollowing")
}

// TestPersonnelFollow_MissingPersonnelIsNotFound — following a missing or
// inactive professional is the project's canonical 404, never a silent success.
func TestPersonnelFollow_MissingPersonnelIsNotFound(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := authzTestRouter(t)

	seedDashboardCenter(t, db, 1, "Follow Center", "follow@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, false) // inactive
	seedClient(t, db, 5, "follower", "follower@test.com")

	token := roleToken(t, 5, 0, "client")

	w := do(r, http.MethodPost, "/api/personnel/999/follow", token, "")
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	w = do(r, http.MethodPost, "/api/personnel/2/follow", token, "")
	assert.Equal(t, http.StatusNotFound, w.Code, "inactive professional must be 404")
}

// TestPersonnelFollow_ClientOnly — the role gate rejects non-client tokens.
func TestPersonnelFollow_ClientOnly(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := authzTestRouter(t)

	seedDashboardCenter(t, db, 1, "Follow Center", "follow@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)

	w := do(r, http.MethodPost, "/api/personnel/1/follow", roleToken(t, 1, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusForbidden, w.Code)
}
