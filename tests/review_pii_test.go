package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// reviewAuthzTestRouter mounts ReviewRoutes the way router.go does, over the
// shared SQLite schema.
func reviewAuthzTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	r := gin.New()
	r.Use(gin.Recovery())
	router.ReviewRoutes(r.Group("/api"), db, validator.New())
	return r, db
}

// seedReviewFixtures creates center 1 with two reviews by two clients whose
// email/phone/username are distinctive, so a leak is detectable on the raw
// wire body.
func seedReviewFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedDashboardCenter(t, db, 1, "Review Center", "review-center@test.com", true)

	c1 := &models.Client{
		Model:     gorm.Model{ID: 1},
		FirstName: "Ana",
		LastName:  "Lopez",
		Username:  "ana-secret",
		Phone:     "9990001",
		Email:     "ana-secret@test.com",
		AvatarURL: "https://cdn.test/ana.png",
		IsActive:  true,
	}
	c2 := &models.Client{
		Model:     gorm.Model{ID: 2},
		FirstName: "Beto",
		LastName:  "Diaz",
		Username:  "beto-secret",
		Phone:     "9990002",
		Email:     "beto-secret@test.com",
		IsActive:  true,
	}
	if err := db.Create(c1).Error; err != nil {
		t.Fatalf("failed to seed client 1: %v", err)
	}
	if err := db.Create(c2).Error; err != nil {
		t.Fatalf("failed to seed client 2: %v", err)
	}
	if err := db.Create(&models.Review{Model: gorm.Model{ID: 1}, ClientID: 1, BeautyCenterID: 1, Rating: 5, Comment: "Great"}).Error; err != nil {
		t.Fatalf("failed to seed review 1: %v", err)
	}
	if err := db.Create(&models.Review{Model: gorm.Model{ID: 2}, ClientID: 2, BeautyCenterID: 1, Rating: 4, Comment: "Good"}).Error; err != nil {
		t.Fatalf("failed to seed review 2: %v", err)
	}
}

// reviewListClientObjects parses a {"data":[...]} response body and returns each
// row's decoded `client` object. Parsing the RAW JSON (not the Go struct) is the
// point: a PII field present in the struct but absent from the wire — or vice
// versa — must be caught.
func reviewListClientObjects(t *testing.T, body string) []map[string]any {
	t.Helper()
	var envelope struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatalf("response is not the expected {\"data\":[...]} envelope: %v (body=%s)", err, body)
	}
	clients := make([]map[string]any, 0, len(envelope.Data))
	for _, row := range envelope.Data {
		raw, ok := row["client"]
		if !ok {
			clients = append(clients, nil)
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatalf("client is not a JSON object: %v", err)
		}
		clients = append(clients, obj)
	}
	return clients
}

// assertNoClientPII fails if a decoded client object carries email/phone/
// username, and requires the non-PII fields the UI actually renders.
func assertNoClientPII(t *testing.T, label string, c map[string]any) {
	t.Helper()
	if !assert.NotNil(t, c, "%s: client object missing", label) {
		return
	}
	for _, key := range []string{"email", "phone", "username"} {
		_, present := c[key]
		assert.Falsef(t, present, "%s: client must not leak %q (got body keys %v)", label, key, c)
	}
	assert.Containsf(t, c, "first_name", "%s: first_name must remain", label)
	assert.Containsf(t, c, "last_name", "%s: last_name must remain", label)
}

// TestReviewPublicCenterResponse_NoClientPII — GET
// /reviews/beauty-center/:id stays PUBLIC (clients browsing a center need it),
// but the embedded client must no longer expose email/phone/username. The
// assertion reads the actual JSON keys, not the Go struct.
func TestReviewPublicCenterResponse_NoClientPII(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reviewAuthzTestRouter(t)
	seedReviewFixtures(t, db)

	w := do(r, http.MethodGet, "/api/reviews/beauty-center/1", "", "")

	assert.Equal(t, http.StatusOK, w.Code,
		"clients browsing a center's reviews must still work anonymously")

	clients := reviewListClientObjects(t, w.Body.String())
	assert.Len(t, clients, 2)
	for i, c := range clients {
		assertNoClientPII(t, "center review", c)
		_ = i
	}

	// Belt and braces: the seeded secret values must not appear anywhere.
	for _, secret := range []string{"ana-secret@test.com", "beto-secret@test.com", "9990001", "9990002", "ana-secret", "beto-secret"} {
		assert.NotContainsf(t, w.Body.String(), secret,
			"raw public review body must not contain PII %q", secret)
	}
}

// TestReviewPublicSingleResponse_NoClientPII — GET /reviews/:id is public too
// and shares the same trimmed client shape.
func TestReviewPublicSingleResponse_NoClientPII(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reviewAuthzTestRouter(t)
	seedReviewFixtures(t, db)

	w := do(r, http.MethodGet, "/api/reviews/1", "", "")
	assert.Equal(t, http.StatusOK, w.Code)

	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))

	var client map[string]any
	assert.NoError(t, json.Unmarshal(envelope.Data["client"], &client))
	assertNoClientPII(t, "single review", client)
	assert.Equal(t, "Ana", client["first_name"])
	assert.Equal(t, "Lopez", client["last_name"])
	assert.NotContains(t, w.Body.String(), "ana-secret@test.com")
	assert.NotContains(t, w.Body.String(), "9990001")
}

// TestReviewClientList_AuthAndOwnership — GET /reviews/client/:clientID was
// public, allowing anyone to enumerate client ids and harvest their review
// history. It now requires a token and applies the loyalty ownership rule: a
// `client` token may read only its own id (403 otherwise); beauty_center and
// personnel keep the legitimate staff-facing read.
func TestReviewClientList_AuthAndOwnership(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reviewAuthzTestRouter(t)
	seedReviewFixtures(t, db)

	t.Run("anonymous is 401", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/reviews/client/1", "", "")

		assert.Equal(t, http.StatusUnauthorized, w.Code,
			"the client review list must not be enumerable anonymously")
		assert.Equal(t, "Authorization header missing", errBody(t, w.Body.String()))
	})

	t.Run("a client reading another client's reviews is 403", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/reviews/client/1", roleToken(t, 9, 0, "client"), "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
		assert.NotContains(t, w.Body.String(), "Great",
			"no review data may leak to the rejected caller")
	})

	t.Run("the owner client reads its own reviews", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/reviews/client/1", roleToken(t, 1, 0, "client"), "")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"comment":"Great"`)
	})

	t.Run("beauty_center admin reads a client's reviews", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/reviews/client/1", roleToken(t, 7, 1, "beauty_center"), "")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"comment":"Great"`)
	})

	t.Run("personnel reads a client's reviews", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/reviews/client/1", roleToken(t, 7, 1, "personnel"), "")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"comment":"Great"`)
	})
}

// reviewUpdateBody is a valid UpdateReviewRequest payload: it passes binding and
// validation, so the ownership probe actually reaches the service instead of
// being short-circuited by a 400.
const reviewUpdateBody = `{"rating":4}`

// TestReviewUpdate_OwnershipIsForbidden — PUT /reviews/:id on a review owned by
// ANOTHER client. The service enforces ownership, but the controller used to map
// every service error to 500 and echo err.Error(), so the caller got 500 plus the
// internal authorization message. It must be 403 {"error":"forbidden"} with no
// internal text on the wire.
func TestReviewUpdate_OwnershipIsForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reviewAuthzTestRouter(t)
	seedReviewFixtures(t, db)

	// review 1 belongs to client 1; client 2 must be rejected.
	w := do(r, http.MethodPut, "/api/reviews/1", roleToken(t, 2, 0, "client"), reviewUpdateBody)

	assert.Equal(t, http.StatusForbidden, w.Code,
		"updating another client's review must be forbidden, never a 500")
	assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	assert.NotContains(t, w.Body.String(), "unauthorized",
		"the internal authorization message must not leak")
	assert.NotContains(t, w.Body.String(), "you can only update your own reviews")
}

// TestReviewDelete_OwnershipIsForbidden — DELETE /reviews/:id on a review owned
// by ANOTHER client must be 403 with a leak-free body.
func TestReviewDelete_OwnershipIsForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reviewAuthzTestRouter(t)
	seedReviewFixtures(t, db)

	// review 2 belongs to client 2; client 1 must be rejected.
	w := do(r, http.MethodDelete, "/api/reviews/2", roleToken(t, 1, 0, "client"), "")

	assert.Equal(t, http.StatusForbidden, w.Code,
		"deleting another client's review must be forbidden, never a 500")
	assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	assert.NotContains(t, w.Body.String(), "unauthorized",
		"the internal authorization message must not leak")
	assert.NotContains(t, w.Body.String(), "you can only delete your own reviews")
}

// TestReviewUpdateDelete_OwnReviewSucceeds — the success paths must keep their
// previous status and response bodies unchanged by the error-mapping fix.
func TestReviewUpdateDelete_OwnReviewSucceeds(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reviewAuthzTestRouter(t)
	seedReviewFixtures(t, db)

	upd := do(r, http.MethodPut, "/api/reviews/1", roleToken(t, 1, 0, "client"), reviewUpdateBody)
	assert.Equal(t, http.StatusOK, upd.Code)
	assert.JSONEq(t, `{"message":"review updated successfully"}`, upd.Body.String())

	del := do(r, http.MethodDelete, "/api/reviews/2", roleToken(t, 2, 0, "client"), "")
	assert.Equal(t, http.StatusOK, del.Code)
	assert.JSONEq(t, `{"message":"review deleted successfully"}`, del.Body.String())
}

// TestReviewUpdateDelete_MissingReviewIsNotFound — a non-existent review id must
// follow the codebase's not-found convention: 404 {"error":"not found"} from
// apperrors.ErrNotFound via MapToHTTP (same as loyalty/promotion services).
func TestReviewUpdateDelete_MissingReviewIsNotFound(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reviewAuthzTestRouter(t)
	seedReviewFixtures(t, db)

	put := do(r, http.MethodPut, "/api/reviews/9999", roleToken(t, 1, 0, "client"), reviewUpdateBody)
	assert.Equal(t, http.StatusNotFound, put.Code)
	assert.Equal(t, "not found", errBody(t, put.Body.String()))

	del := do(r, http.MethodDelete, "/api/reviews/9999", roleToken(t, 1, 0, "client"), "")
	assert.Equal(t, http.StatusNotFound, del.Code)
	assert.Equal(t, "not found", errBody(t, del.Body.String()))
}
