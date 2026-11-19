package tests

// Coverage for session-bound access tokens (C2): a token stops authenticating as
// soon as its session is revoked, instead of staying valid until the token's
// 15-minute expiry.
//
// The two properties that matter and pull in opposite directions:
//   - a real revocation (logout-all, password reset, deactivation, cap
//     eviction) is visible on the NEXT request;
//   - a rotation (refresh) does not 401 the request the client still has in
//     flight with the previous access token.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/middleware"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ─── middleware gate ────────────────────────────────────────────────────────

// fakeChecker is the minimal SessionActivityChecker for middleware tests.
type fakeChecker struct {
	active bool
	err    error
	calls  int
	lastID uint
}

func (f *fakeChecker) IsActive(_ context.Context, sessionID uint, _ time.Time, _ time.Duration) (bool, error) {
	f.calls++
	f.lastID = sessionID
	return f.active, f.err
}

// gatedRouter mounts the real middleware over a probe handler.
func gatedRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.JWTAuthMiddleware())
	r.GET("/probe", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"ok": true, "userID": ctx.GetUint("clientID")})
	})
	return r
}

func probe(r *gin.Engine, token string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(http.MethodGet, "/probe", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSessionGate_RejectsRevokedSession(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	t.Cleanup(func() { middleware.SetSessionChecker(nil) })

	token, err := utils.GenerateJWT(5, 0, "client", "a@b.com", "ana", "77")
	require.NoError(t, err)

	checker := &fakeChecker{active: true}
	middleware.SetSessionChecker(checker)

	w := probe(gatedRouter(), token)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, uint(77), checker.lastID, "the gate must verify the token's own sid")

	// The session is revoked server-side: the SAME token is now rejected, with
	// no need to wait for the 15-minute expiry.
	checker.active = false
	w = probe(gatedRouter(), token)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSessionGate_StoreFailureIs503NotCredentialFailure(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	t.Cleanup(func() { middleware.SetSessionChecker(nil) })

	token, err := utils.GenerateJWT(5, 0, "client", "a@b.com", "ana", "77")
	require.NoError(t, err)

	// Fails CLOSED (access denied) but with 503, so the client does not treat an
	// infrastructure blip as "your session expired" and wipe its credentials.
	middleware.SetSessionChecker(&fakeChecker{err: errors.New("db down")})

	w := probe(gatedRouter(), token)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "authentication temporarily unavailable")
}

func TestSessionGate_RejectsTokenWithoutSessionID(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	t.Cleanup(func() { middleware.SetSessionChecker(nil) })

	// A token minted before this change (or hand-rolled) has no sid and cannot be
	// revoked, so it must not authenticate under an enabled checker.
	token, err := utils.GenerateJWT(5, 0, "client", "a@b.com", "ana", "")
	require.NoError(t, err)

	checker := &fakeChecker{active: true}
	middleware.SetSessionChecker(checker)

	w := probe(gatedRouter(), token)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Zero(t, checker.calls, "a token without a sid is rejected before touching the store")
}

// ─── repository semantics ───────────────────────────────────────────────────

// sessionRepo returns a repository and the DB it reads from. Both must come from
// the SAME handle: :memory: SQLite databases are per-connection, so building a
// second one would query an empty database.
func sessionRepo(t *testing.T) (repository.UserSessionRepository, *gorm.DB) {
	t.Helper()
	db := setupRouterAuthzDB(t)
	return repository.NewUserSessionRepositoryImpl(db), db
}

func newSessionRow(t *testing.T, db *gorm.DB, family string) *models.UserSession {
	t.Helper()
	s := &models.UserSession{
		UserID:           1,
		FamilyID:         family,
		RefreshTokenHash: family + "-hash",
		ExpiresAt:        time.Now().Add(time.Hour),
		Role:             "client",
	}
	require.NoError(t, db.Create(s).Error)
	return s
}

func TestSessionIsActive_RevocationIsImmediate(t *testing.T) {
	repo, db := sessionRepo(t)
	ctx := context.Background()

	s := newSessionRow(t, db, "fam-1")

	active, err := repo.IsActive(ctx, s.ID, time.Now(), 30*time.Second)
	require.NoError(t, err)
	assert.True(t, active)

	require.NoError(t, repo.RevokeAllForUser(ctx, 1, "client"))

	active, err = repo.IsActive(ctx, s.ID, time.Now(), 30*time.Second)
	require.NoError(t, err)
	assert.False(t, active, "a revoked session must be rejected on the next request even inside the grace window")
}

func TestSessionIsActive_ExpiredSessionIsInactive(t *testing.T) {
	repo, db := sessionRepo(t)
	ctx := context.Background()

	s := newSessionRow(t, db, "fam-exp")
	require.NoError(t, db.Model(s).Update("expires_at", time.Now().Add(-time.Minute)).Error)

	active, err := repo.IsActive(ctx, s.ID, time.Now(), 30*time.Second)
	require.NoError(t, err)
	assert.False(t, active)
}

// TestSessionIsActive_RotationGrace — refresh revokes the presented generation
// and inserts the next one in the same family. The old access token must keep
// working for the grace window (so an in-flight request is not 401'd) and stop
// as soon as the window closes.
func TestSessionIsActive_RotationGrace(t *testing.T) {
	repo, db := sessionRepo(t)
	ctx := context.Background()

	oldGen := newSessionRow(t, db, "fam-rot")

	// Simulate the rotation transaction: revoke the old generation, insert the
	// new one in the same family.
	require.NoError(t, db.Model(oldGen).Update("is_revoked", true).Error)
	newGen := newSessionRow(t, db, "fam-rot")

	now := time.Now()
	active, err := repo.IsActive(ctx, oldGen.ID, now, 30*time.Second)
	require.NoError(t, err)
	assert.True(t, active, "a just-rotated generation stays usable inside the grace window")

	// The new generation is unconditionally active.
	active, err = repo.IsActive(ctx, newGen.ID, now, 30*time.Second)
	require.NoError(t, err)
	assert.True(t, active)

	// Past the grace window the old generation is done.
	active, err = repo.IsActive(ctx, oldGen.ID, now.Add(2*time.Minute), 30*time.Second)
	require.NoError(t, err)
	assert.False(t, active)

	// And revoking the family kills the grace immediately: the family no longer
	// holds a live generation.
	active, err = repo.IsActive(ctx, oldGen.ID, now, 30*time.Second)
	require.NoError(t, err)
	require.True(t, active)
	require.NoError(t, repo.RevokeFamily(ctx, "fam-rot"))
	active, err = repo.IsActive(ctx, oldGen.ID, now, 30*time.Second)
	require.NoError(t, err)
	assert.False(t, active)
}

func TestSessionIsActive_UnknownSessionIsInactive(t *testing.T) {
	repo, _ := sessionRepo(t)

	active, err := repo.IsActive(context.Background(), 999, time.Now(), 30*time.Second)
	require.NoError(t, err)
	assert.False(t, active)
}

// ─── token/session binding ──────────────────────────────────────────────────

func TestGeneratedTokenCarriesItsSessionID(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)

	token, err := utils.GenerateJWT(3, 0, "client", "a@b.com", "ana", "1234")
	require.NoError(t, err)

	claims, err := utils.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, "1234", claims.SessionID)
	assert.Equal(t, strconv.FormatUint(1234, 10), claims.SessionID)
}

// TestSetupRouterWiresSessionChecker — the gate is only as good as its wiring.
// A nil checker accepts every token, so this asserts the real router installs
// one instead of relying on a comment.
func TestSetupRouterWiresSessionChecker(t *testing.T) {
	gin.SetMode(gin.TestMode)
	utils.Log = zap.NewNop().Sugar()
	middleware.SetSessionChecker(nil)
	t.Cleanup(func() { middleware.SetSessionChecker(nil) })

	_ = router.SetupRouter()

	assert.True(t, middleware.SessionCheckerWired(),
		"SetupRouter must install a session checker; a nil one silently disables revocation")
}
