package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  C1 — client comments must be attributed to their author
// ──────────────────────────────────────────────

// setupCommentRouter mounts the PRODUCTION VideoCommentRoutes so the regression
// exercises the real composition root over the shared SQLite schema.
func setupCommentRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	r := gin.New()
	router.VideoCommentRoutes(r.Group("/api"), db, validator.New())
	return r, db
}

func seedCommentPost(t *testing.T, db *gorm.DB) *models.FeedPost {
	t.Helper()
	seedDashboardCenter(t, db, 3, "Comment Center", "comment@test.com", true)
	post := &models.FeedPost{
		BeautyCenterID: 3,
		VideoURL:       "/videos/comment.mp4",
		ThumbnailURL:   "/thumbs/comment.jpg",
		Caption:        "balayage",
		IsActive:       true,
	}
	require.NoError(t, db.Create(post).Error)
	return post
}

// TestComment_ClientCommentIsAttributedAndOwned is the regression for the
// `actor.Role == ""` bug: a client writes a comment with role "client", so the
// old predicate never fired, client_id stayed NULL, the author rendered empty,
// and update/delete returned 403 because ownership compares client_id.
func TestComment_ClientCommentIsAttributedAndOwned(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupCommentRouter(t)
	post := seedCommentPost(t, db)
	seedClient(t, db, 5, "commenter", "commenter@test.com")

	token := roleToken(t, 5, 0, "client")
	base := fmt.Sprintf("/api/feed-posts/%d/comments", post.ID)

	// Create — must persist client_id and surface the author.
	w := do(r, http.MethodPost, base, token, `{"body":"Hermoso trabajo"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var created response.CommentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, uint(5), created.Author.ID, "author id must be the commenting client")
	assert.Equal(t, "Test Client", created.Author.Name, "author name must resolve from the client row")

	var stored models.VideoComment
	require.NoError(t, db.First(&stored, created.ID).Error)
	require.NotNil(t, stored.ClientID, "client_id must be persisted, not NULL")
	assert.Equal(t, uint(5), *stored.ClientID)

	// The author can edit their own comment (was 403 before the fix).
	w = do(r, http.MethodPatch, fmt.Sprintf("%s/%d", base, created.ID), token, `{"body":"editado"}`)
	assert.Equal(t, http.StatusOK, w.Code, "author must be able to update own comment: %s", w.Body.String())

	// The author can delete their own comment (was 403 before the fix).
	w = do(r, http.MethodDelete, fmt.Sprintf("%s/%d", base, created.ID), token, "")
	assert.Equal(t, http.StatusOK, w.Code, "author must be able to delete own comment: %s", w.Body.String())
}

// TestComment_CenterCommentHasNoClientID proves a non-client comment is NOT
// attributed to a client: the center replies as the center, so client_id must
// stay NULL.
func TestComment_CenterCommentHasNoClientID(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupCommentRouter(t)
	post := seedCommentPost(t, db)

	base := fmt.Sprintf("/api/feed-posts/%d/comments", post.ID)
	w := do(r, http.MethodPost, base, roleToken(t, 1, 3, "beauty_center"), `{"body":"Gracias por tu visita"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var created response.CommentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	var stored models.VideoComment
	require.NoError(t, db.First(&stored, created.ID).Error)
	assert.Nil(t, stored.ClientID, "a center comment must not be attributed to a client")
	assert.False(t, stored.IsOfficial)
}
