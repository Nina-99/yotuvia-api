package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
//  N3 — official reply / hide are scoped to the post's owning center
// ──────────────────────────────────────────────

// TestComment_CrossCenterReplyAndHideRejected proves a center cannot post an
// OFFICIAL reply, nor hide a comment, on another center's video. Before the
// fix ReplyToComment/HideComment only checked `actor.CenterID == 0`, so any
// authenticated center could act on any post. DeleteComment already enforced
// ownership and is the reference pattern.
func TestComment_CrossCenterReplyAndHideRejected(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupCommentRouter(t)
	post := seedCommentPost(t, db) // belongs to center 3
	seedClient(t, db, 5, "commenter", "commenter@test.com")

	base := fmt.Sprintf("/api/feed-posts/%d/comments", post.ID)

	// A client writes the comment both centers will target.
	w := do(r, http.MethodPost, base, roleToken(t, 5, 0, "client"), `{"body":"hermoso"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var created response.CommentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	replyPath := fmt.Sprintf("%s/%d/reply", base, created.ID)

	// FOREIGN center 1 tries to post an official reply → rejected, nothing stored.
	w = do(r, http.MethodPost, replyPath, roleToken(t, 7, 1, "beauty_center"), `{"body":"oficial"}`)
	assert.NotEqual(t, http.StatusCreated, w.Code, "foreign center must not create an official reply")
	assert.NotContains(t, w.Body.String(), `"is_official":true`)

	var officialCount int64
	require.NoError(t, db.Model(&models.VideoComment{}).
		Where("parent_id = ? AND is_official = ?", created.ID, true).
		Count(&officialCount).Error)
	assert.Equal(t, int64(0), officialCount, "no official reply row must be persisted for a foreign center")

	// OWNING center 3 is still allowed.
	w = do(r, http.MethodPost, replyPath, roleToken(t, 7, 3, "beauty_center"), `{"body":"gracias"}`)
	assert.Equal(t, http.StatusCreated, w.Code, "owning center must still be allowed to reply: %s", w.Body.String())

	hidePath := fmt.Sprintf("%s/%d/hide", base, created.ID)

	// FOREIGN center tries to hide → 403, comment stays visible.
	w = do(r, http.MethodPatch, hidePath, roleToken(t, 7, 1, "beauty_center"), "")
	assert.Equal(t, http.StatusForbidden, w.Code, "foreign center must not hide: %s", w.Body.String())

	var stored models.VideoComment
	require.NoError(t, db.First(&stored, created.ID).Error)
	assert.False(t, stored.IsHidden, "comment must remain visible after a foreign hide attempt")

	// OWNING center hides → 200.
	w = do(r, http.MethodPatch, hidePath, roleToken(t, 7, 3, "beauty_center"), "")
	assert.Equal(t, http.StatusOK, w.Code, "owning center must be allowed to hide: %s", w.Body.String())

	require.NoError(t, db.First(&stored, created.ID).Error)
	assert.True(t, stored.IsHidden, "owning center's hide must persist")
}
