package tests

import (
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

// favoriteTestRouter mounts FavoriteRoutes the way router.go does, over the
// shared SQLite schema. Recovery keeps a handler panic from crashing the binary
// so it surfaces as a 500 assertion failure instead.
func favoriteTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	r := gin.New()
	r.Use(gin.Recovery())
	router.FavoriteRoutes(r.Group("/api"), db, validator.New())
	return r, db
}

// seedFavoriteFixtures creates two beauty centers and one favorite per client
// (client 1 owns favorite 1, client 2 owns favorite 2), so IDOR probes target a
// row that genuinely belongs to somebody else. An IDOR probe is only meaningful
// when the target is a real foreign row.
func seedFavoriteFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedDashboardCenter(t, db, 1, "Favorite Center One", "fav1@test.com", true)
	seedDashboardCenter(t, db, 2, "Favorite Center Two", "fav2@test.com", true)
	if err := db.Create(&models.Favorite{
		Model:          gorm.Model{ID: 1},
		ClientID:       1,
		BeautyCenterID: 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed favorite 1: %v", err)
	}
	if err := db.Create(&models.Favorite{
		Model:          gorm.Model{ID: 2},
		ClientID:       2,
		BeautyCenterID: 2,
	}).Error; err != nil {
		t.Fatalf("failed to seed favorite 2: %v", err)
	}
}

// TestFavoriteAuthz_NoTokenIs401 — every /favorites and /clients/:id/favorites
// route must reject anonymous callers with the JWT middleware's response and
// never reach its handler.
func TestFavoriteAuthz_NoTokenIs401(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, _ := favoriteTestRouter(t)

	routes := []struct {
		name   string
		method string
		path   string
	}{
		{"add favorite", http.MethodPost, "/api/favorites"},
		{"remove favorite", http.MethodDelete, "/api/favorites/1"},
		{"client favorites", http.MethodGet, "/api/clients/1/favorites"},
	}

	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			w := do(r, rt.method, rt.path, "", "")

			assert.Equal(t, http.StatusUnauthorized, w.Code,
				"%s %s must reject anonymous callers", rt.method, rt.path)
			assert.Equal(t, "Authorization header missing", errBody(t, w.Body.String()),
				"handler must not run for an anonymous request")
		})
	}
}

// TestFavoriteDelete_CrossOwnerForbidden — IDOR closed for DELETE. A `client`
// token must not be able to remove a favorite whose ClientID is not its own,
// and the foreign row must survive byte-for-byte. The legitimate owner still
// removes its own favorite.
func TestFavoriteDelete_CrossOwnerForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := favoriteTestRouter(t)
	seedFavoriteFixtures(t, db)

	// Caller is client 9; favorite 1 belongs to client 1.
	w := do(r, http.MethodDelete, "/api/favorites/1", roleToken(t, 9, 0, "client"), "")

	assert.Equal(t, http.StatusForbidden, w.Code,
		"a client must not delete another client's favorite")
	assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	assert.NotContains(t, w.Body.String(), "favorite removed",
		"the success message must never appear for a rejected owner")

	var count int64
	assert.NoError(t, db.Model(&models.Favorite{}).Where("id = ?", 1).Count(&count).Error)
	assert.Equal(t, int64(1), count, "another client's favorite must survive the rejected delete")

	// The owner still removes its own favorite.
	w = do(r, http.MethodDelete, "/api/favorites/1", roleToken(t, 1, 0, "client"), "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "favorite removed successfully")

	assert.NoError(t, db.Model(&models.Favorite{}).Where("id = ?", 1).Count(&count).Error)
	assert.Equal(t, int64(0), count, "the owner's delete removes the row")
}

// TestFavoriteDelete_MissingIdIs404 — a nonexistent favorite follows the
// codebase's not-found convention (404 {"error":"not found"}), never a 500 and
// never a silent 200 from a no-op GORM delete.
func TestFavoriteDelete_MissingIdIs404(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := favoriteTestRouter(t)
	seedFavoriteFixtures(t, db)

	w := do(r, http.MethodDelete, "/api/favorites/9999", roleToken(t, 1, 0, "client"), "")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "not found", errBody(t, w.Body.String()))
}

// TestFavoriteDelete_StaffForbidden — favorites are a client-only, private
// surface. A staff token (beauty_center/personnel) must not delete a client's
// favorite even when its polymorphic user_id claim collides with a client id,
// and the row must survive.
func TestFavoriteDelete_StaffForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := favoriteTestRouter(t)
	seedFavoriteFixtures(t, db)

	for _, role := range []string{"beauty_center", "personnel"} {
		t.Run(role, func(t *testing.T) {
			// userID 1 collides with the owning client's id on purpose.
			w := do(r, http.MethodDelete, "/api/favorites/1", roleToken(t, 1, 1, role), "")

			assert.Equal(t, http.StatusForbidden, w.Code,
				"%s must not delete a client's favorite", role)
			assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
		})
	}

	var count int64
	assert.NoError(t, db.Model(&models.Favorite{}).Where("id = ?", 1).Count(&count).Error)
	assert.Equal(t, int64(1), count, "the favorite must survive every staff attempt")
}

// TestFavoriteRead_CrossClientForbidden — IDOR closed for the read. A `client`
// token may only read the favorites of the clientID in its own claim; asking for
// another client's :client_id is 403 and must not leak a single row. The owner
// still reads its own list.
func TestFavoriteRead_CrossClientForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := favoriteTestRouter(t)
	seedFavoriteFixtures(t, db)

	// Caller is client 9; the requested list belongs to client 1.
	w := do(r, http.MethodGet, "/api/clients/1/favorites", roleToken(t, 9, 0, "client"), "")

	assert.Equal(t, http.StatusForbidden, w.Code,
		"a client must not read another client's favorites")
	assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	assert.NotContains(t, w.Body.String(), "Favorite Center One",
		"no favorite data may leak to the rejected caller")

	// The owner still reads its own favorites unchanged.
	w = do(r, http.MethodGet, "/api/clients/1/favorites", roleToken(t, 1, 0, "client"), "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Favorite Center One")
	// ...and never sees the other client's row.
	assert.NotContains(t, w.Body.String(), "Favorite Center Two")
}

// TestFavoriteRead_StaffForbidden — pins the staff policy: there is no staff
// consumer of a client's favorites (web ClientFavoritesPage is client-role-only;
// mobile routes staff away from the client area), so beauty_center and personnel
// are rejected with 403 even when their polymorphic user_id claim collides with
// the target client's id.
func TestFavoriteRead_StaffForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := favoriteTestRouter(t)
	seedFavoriteFixtures(t, db)

	for _, role := range []string{"beauty_center", "personnel"} {
		t.Run(role, func(t *testing.T) {
			w := do(r, http.MethodGet, "/api/clients/1/favorites", roleToken(t, 1, 1, role), "")

			assert.Equal(t, http.StatusForbidden, w.Code,
				"%s must not read a client's favorites", role)
			assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
			assert.NotContains(t, w.Body.String(), "Favorite Center One")
		})
	}
}

// TestFavoriteAuthz_MalformedIdStillBadRequest — the path-parse guard is
// unchanged: a non-numeric :id / :client_id stays 400 with a fixed message, not
// a 500 and not an internal error echo.
func TestFavoriteAuthz_MalformedIdStillBadRequest(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := favoriteTestRouter(t)
	seedFavoriteFixtures(t, db)

	w := do(r, http.MethodDelete, "/api/favorites/not-a-number", roleToken(t, 1, 0, "client"), "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "invalid id", errBody(t, w.Body.String()))

	w = do(r, http.MethodGet, "/api/clients/not-a-number/favorites", roleToken(t, 1, 0, "client"), "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "invalid client_id", errBody(t, w.Body.String()))
}
