package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/router"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
//  Comment list — replies populated + batched lookups
// ──────────────────────────────────────────────

// fetchCommentList issues GET /feed-posts/:id/comments and decodes the payload.
func fetchCommentList(t *testing.T, r *gin.Engine, postID uint) response.CommentListResponse {
	t.Helper()
	w := do(r, http.MethodGet, fmt.Sprintf("/api/feed-posts/%d/comments", postID), roleToken(t, 5, 0, "client"), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var list response.CommentListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	return list
}

// TestCommentList_RepliesNestedInAscendingOrder proves the frozen contract:
// GET /feed-posts/:id/comments returns each top-level comment with its replies
// nested under `replies`, ordered created_at ASC, and nothing nests beyond one
// level. Before the fix FindByFeedPost had no Preload("Replies"), so the list
// always returned an empty replies array even though the single-comment paths
// (which use FindByID) looked fine.
func TestCommentList_RepliesNestedInAscendingOrder(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupCommentRouter(t)
	post := seedCommentPost(t, db) // owned by center 3
	cl := seedClient(t, db, 5, "commenter", "commenter@test.com")
	require.NoError(t, db.Model(cl).Update("avatar_url", "https://cdn.test/avatar.png").Error)

	base := fmt.Sprintf("/api/feed-posts/%d/comments", post.ID)

	// Top-level comment written through the API.
	w := do(r, http.MethodPost, base, roleToken(t, 5, 0, "client"), `{"body":"parent"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var parent response.CommentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parent))

	// Official reply from the owning center.
	w = do(r, http.MethodPost, fmt.Sprintf("%s/%d/reply", base, parent.ID),
		roleToken(t, 7, 3, "beauty_center"), `{"body":"oficial"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var official response.CommentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &official))

	// A client-authored reply, seeded directly (no API path allows client
	// replies) and back-dated so ASC ordering is provable.
	clientReply := &models.VideoComment{
		FeedPostID: post.ID,
		ParentID:   &parent.ID,
		ClientID:   &cl.ID,
		Body:       "client-reply",
		CreatedAt:  time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(clientReply).Error)

	list := fetchCommentList(t, r, post.ID)
	require.Len(t, list.Comments, 1, "the single top-level comment must be the only root")

	top := list.Comments[0]
	assert.Equal(t, parent.ID, top.ID)
	require.Len(t, top.Replies, 2, "both replies must be nested under the parent")

	// ASC by created_at: the back-dated client reply comes first.
	assert.Equal(t, clientReply.ID, top.Replies[0].ID, "earlier reply must come first (ASC)")
	assert.Equal(t, official.ID, top.Replies[1].ID)

	// Only the parent carries replies — no nesting beyond one level.
	for _, rep := range top.Replies {
		assert.Empty(t, rep.Replies, "reply %d must not carry nested replies", rep.ID)
	}
}

// TestCommentList_AuthorResolvesForCommentAndReply proves the batched author
// lookup resolves name/avatar for a client comment AND a client reply, and the
// owning center for an official reply. Before the fix these ran one SELECT per
// comment (FindById) plus one FindByID per official reply; here a single page
// request still renders every author.
func TestCommentList_AuthorResolvesForCommentAndReply(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupCommentRouter(t)
	post := seedCommentPost(t, db)
	cl := seedClient(t, db, 5, "commenter", "commenter@test.com")
	require.NoError(t, db.Model(cl).Update("avatar_url", "https://cdn.test/avatar.png").Error)

	base := fmt.Sprintf("/api/feed-posts/%d/comments", post.ID)

	w := do(r, http.MethodPost, base, roleToken(t, 5, 0, "client"), `{"body":"parent"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var parent response.CommentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parent))

	w = do(r, http.MethodPost, fmt.Sprintf("%s/%d/reply", base, parent.ID),
		roleToken(t, 7, 3, "beauty_center"), `{"body":"oficial"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	clientReply := &models.VideoComment{
		FeedPostID: post.ID,
		ParentID:   &parent.ID,
		ClientID:   &cl.ID,
		Body:       "client-reply",
	}
	require.NoError(t, db.Create(clientReply).Error)

	list := fetchCommentList(t, r, post.ID)
	require.Len(t, list.Comments, 1)
	top := list.Comments[0]

	// Parent: client author.
	assert.Equal(t, uint(5), top.Author.ID)
	assert.Equal(t, "Test Client", top.Author.Name)
	assert.Equal(t, "https://cdn.test/avatar.png", top.Author.Avatar)

	require.Len(t, top.Replies, 2)

	var clientAuthor, officialAuthor *response.CommentResponse
	for i := range top.Replies {
		if top.Replies[i].IsOfficial {
			officialAuthor = &top.Replies[i]
		} else {
			clientAuthor = &top.Replies[i]
		}
	}

	require.NotNil(t, clientAuthor, "client reply must be present")
	assert.Equal(t, uint(5), clientAuthor.Author.ID)
	assert.Equal(t, "Test Client", clientAuthor.Author.Name)
	assert.Equal(t, "https://cdn.test/avatar.png", clientAuthor.Author.Avatar)

	require.NotNil(t, officialAuthor, "official reply must be present")
	assert.True(t, officialAuthor.Author.IsCenter)
	assert.Equal(t, uint(3), officialAuthor.Author.CenterID)
}

// TestCommentList_KeepsHiddenRowsWithFlag locks the moderation contract: hidden
// comments and replies are still returned, flagged with is_hidden, so the
// client decides what to render. This matches the pre-fix behaviour and
// GetCommentReplies/FindReplies, which never filtered is_hidden either; adding
// the replies preload must not silently change moderation semantics.
func TestCommentList_KeepsHiddenRowsWithFlag(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := setupCommentRouter(t)
	post := seedCommentPost(t, db)
	seedClient(t, db, 5, "commenter", "commenter@test.com")

	base := fmt.Sprintf("/api/feed-posts/%d/comments", post.ID)

	w := do(r, http.MethodPost, base, roleToken(t, 5, 0, "client"), `{"body":"visible-parent"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var parent response.CommentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parent))

	w = do(r, http.MethodPost, fmt.Sprintf("%s/%d/reply", base, parent.ID),
		roleToken(t, 7, 3, "beauty_center"), `{"body":"to-be-hidden"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var reply response.CommentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &reply))

	// The owning center hides the comment; the reply is hidden directly.
	w = do(r, http.MethodPatch, fmt.Sprintf("%s/%d/hide", base, parent.ID), roleToken(t, 7, 3, "beauty_center"), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, db.Model(&models.VideoComment{}).Where("id = ?", reply.ID).Update("is_hidden", true).Error)

	list := fetchCommentList(t, r, post.ID)
	require.Len(t, list.Comments, 1, "hidden top-level comment must still be returned")
	assert.True(t, list.Comments[0].IsHidden, "hidden top-level comment must carry is_hidden=true")
	require.Len(t, list.Comments[0].Replies, 1, "hidden reply must still be returned")
	assert.True(t, list.Comments[0].Replies[0].IsHidden, "hidden reply must carry is_hidden=true")
}

// measureCommentListQueries builds a fresh router over its own SQLite DB, seeds
// `parents` top-level comments each with `repliesPerParent` replies (one
// official + the rest client-authored, to exercise both batch lookups), and
// returns the number of SELECTs the list request executes.
func measureCommentListQueries(t *testing.T, parents, repliesPerParent int) int64 {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	counter := attachQueryCounter(db)

	r := gin.New()
	router.VideoCommentRoutes(r.Group("/api"), db, validator.New())

	post := seedCommentPost(t, db) // center 3
	cl := seedClient(t, db, 5, "commenter", "commenter@test.com")

	for i := 0; i < parents; i++ {
		parent := &models.VideoComment{
			FeedPostID: post.ID,
			ClientID:   &cl.ID,
			Body:       fmt.Sprintf("parent-%d", i),
		}
		require.NoError(t, db.Create(parent).Error)

		for j := 0; j < repliesPerParent; j++ {
			reply := &models.VideoComment{
				FeedPostID: post.ID,
				ParentID:   &parent.ID,
				IsOfficial: j == 0,
				Body:       fmt.Sprintf("reply-%d-%d", i, j),
			}
			if j != 0 {
				reply.ClientID = &cl.ID
			}
			require.NoError(t, db.Create(reply).Error)
		}
	}

	atomic.StoreInt64(counter, 0)
	w := do(r, http.MethodGet, fmt.Sprintf("/api/feed-posts/%d/comments", post.ID), roleToken(t, 5, 0, "client"), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return atomic.LoadInt64(counter)
}

// TestCommentList_QueryCountIsBounded proves the list request's SELECT count is
// bounded and does NOT grow with the number of comments or replies. Before the
// fix mapToResponse ran one FindById per client comment plus one FindByID (with
// its 4 preloads) per official reply — once replies rendered, that multiplied.
// After the fix the whole page resolves with one client IN query and one feed
// post lookups in a constant number of statements.
func TestCommentList_QueryCountIsBounded(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)

	small := measureCommentListQueries(t, 2, 2)
	large := measureCommentListQueries(t, 18, 5)

	t.Logf("comment list SELECTs — 2 parents/2 replies: %d, 18 parents/5 replies: %d", small, large)

	// Absolute bound: a page-sized request stays a handful of statements.
	assert.Lessf(t, large, int64(20),
		"query count must be bounded, got %d SELECTs for 18 parents", large)

	// Non-scaling proof: 9x the parents and 2.5x the replies per parent must not
	// add statements (only the IN-list sizes change, not the query count).
	assert.LessOrEqualf(t, large-small, int64(1),
		"query count must not scale with comment count: 2 parents=%d, 18 parents=%d", small, large)
}
