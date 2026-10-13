package tests

// Concurrent-session cap coverage (option A).
//
// One account+role may keep at most MAX_ACTIVE_SESSIONS active sessions
// (default 2). A session here is one LOGIN: `user_sessions.DeviceInfo` is never
// populated, so a "device" cannot be distinguished and the cap is really keyed
// by (user_id, role).
//
// When a new login would push the account past the cap, the login SUCCEEDS and
// the OLDEST active session(s) are revoked so exactly `limit` remain (newest
// first). The eviction count rides back on the auth response as
// `sessions_evicted` so the client can warn the user that another session was
// signed out.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// authEnvelopeWithEvictions is the additive auth response shape: the D4 token
// pair, the user envelope, plus the new `sessions_evicted` counter.
type authEnvelopeWithEvictions struct {
	Token           string `json:"token"`
	RefreshToken    string `json:"refresh_token"`
	SessionsEvicted int    `json:"sessions_evicted"`
	User            struct {
		ID uint `json:"id"`
	} `json:"user"`
}

const sessionLimitLoginBody = `{"identifier":"ana.lopez@test.com","password":"password123","user_type":"client"}`

// registerClientWithEvictions registers the standard harness client and parses
// the eviction count out of the response.
func registerClientWithEvictions(t *testing.T, r *gin.Engine) authEnvelopeWithEvictions {
	t.Helper()
	body := `{"first_name":"Ana","last_name":"Lopez","email":"ana.lopez@test.com","phone":"5550100","password":"password123","username":"ana","user_type":"client","terms_accepted":true}`
	w := do(r, http.MethodPost, "/api/auth-accounts/register", "", body)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	return parseEvictionEnvelope(t, w)
}

// loginClientWithEvictions signs the harness client in and parses the eviction
// count out of the response.
func loginClientWithEvictions(t *testing.T, r *gin.Engine) authEnvelopeWithEvictions {
	t.Helper()
	w := do(r, http.MethodPost, "/api/auth-accounts/login", "", sessionLimitLoginBody)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return parseEvictionEnvelope(t, w)
}

func parseEvictionEnvelope(t *testing.T, w *httptest.ResponseRecorder) authEnvelopeWithEvictions {
	t.Helper()
	var resp authEnvelopeWithEvictions
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse auth response: %v", err)
	}
	assert.NotEmpty(t, resp.RefreshToken, "auth responses must keep issuing a refresh token")
	return resp
}

// sessionByRawToken loads the session row presenting the given raw token.
func sessionByRawToken(t *testing.T, db *gorm.DB, raw string) models.UserSession {
	t.Helper()
	var s models.UserSession
	if err := db.Where("refresh_token_hash = ?", utils.HashRefreshToken(raw)).First(&s).Error; err != nil {
		t.Fatalf("failed to load session by token: %v", err)
	}
	return s
}

// activeSessionCount counts the LIVE sessions of one (user, role): not revoked
// AND not past expiry.
func activeSessionCount(t *testing.T, db *gorm.DB, userID uint, role string) int64 {
	t.Helper()
	var count int64
	err := db.Model(&models.UserSession{}).
		Where("user_id = ? AND role = ? AND is_revoked = ? AND expires_at > ?", userID, role, false, time.Now()).
		Count(&count).Error
	assert.NoError(t, err)
	return count
}

// TestSessionLimit_KeepsTwoSessionsWithNoEviction — the steady state inside the
// cap: register + one login keep BOTH sessions alive and neither response
// reports an eviction (0 is the normal case).
func TestSessionLimit_KeepsTwoSessionsWithNoEviction(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	reg := registerClientWithEvictions(t, r)
	assert.Equal(t, 0, reg.SessionsEvicted, "a first session must never evict anything")

	second := loginClientWithEvictions(t, r)
	assert.Equal(t, 0, second.SessionsEvicted, "the second session still fits under the cap of 2")
	assert.Equal(t, reg.User.ID, second.User.ID, "both logins must resolve the same account")

	assert.Equal(t, int64(2), activeSessionCount(t, db, reg.User.ID, "client"))
	assert.False(t, sessionByRawToken(t, db, reg.RefreshToken).IsRevoked)
	assert.False(t, sessionByRawToken(t, db, second.RefreshToken).IsRevoked)

	// Both sessions can still rotate.
	assert.Equal(t, http.StatusOK, doRefresh(t, r, reg.RefreshToken).Code)
	assert.Equal(t, http.StatusOK, doRefresh(t, r, second.RefreshToken).Code)
}

// TestSessionLimit_ThirdLoginEvictsOldest — the third login SUCCEEDS, reports
// exactly ONE eviction, and the OLDEST session is the one revoked; the two
// newest keep working. The evicted refresh token is dead: presenting it hits
// the reuse path and answers 401.
func TestSessionLimit_ThirdLoginEvictsOldest(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	first := registerClientWithEvictions(t, r)
	second := loginClientWithEvictions(t, r)
	third := loginClientWithEvictions(t, r)

	assert.Equal(t, 1, third.SessionsEvicted, "the third session must report exactly one eviction")
	assert.Equal(t, int64(2), activeSessionCount(t, db, first.User.ID, "client"), "the cap must hold at 2")

	assert.True(t, sessionByRawToken(t, db, first.RefreshToken).IsRevoked,
		"the OLDEST session must be evicted")
	assert.False(t, sessionByRawToken(t, db, second.RefreshToken).IsRevoked)
	assert.False(t, sessionByRawToken(t, db, third.RefreshToken).IsRevoked)

	// The revoked (evicted) session can no longer mint an access token.
	assert.Equal(t, http.StatusUnauthorized, doRefresh(t, r, first.RefreshToken).Code,
		"an evicted session must not be able to refresh")
	// The two survivors still rotate.
	assert.Equal(t, http.StatusOK, doRefresh(t, r, second.RefreshToken).Code)
	assert.Equal(t, http.StatusOK, doRefresh(t, r, third.RefreshToken).Code)
}

// TestSessionLimit_FourthLoginEvictsNextOldest — steady state: every further
// login evicts exactly the next-oldest session and the live count stays at 2.
func TestSessionLimit_FourthLoginEvictsNextOldest(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	first := registerClientWithEvictions(t, r)
	second := loginClientWithEvictions(t, r)
	third := loginClientWithEvictions(t, r)
	fourth := loginClientWithEvictions(t, r)

	assert.Equal(t, 1, fourth.SessionsEvicted)
	assert.Equal(t, int64(2), activeSessionCount(t, db, first.User.ID, "client"))

	assert.True(t, sessionByRawToken(t, db, first.RefreshToken).IsRevoked, "first evicted at login 3")
	assert.True(t, sessionByRawToken(t, db, second.RefreshToken).IsRevoked, "second evicted at login 4")
	assert.False(t, sessionByRawToken(t, db, third.RefreshToken).IsRevoked, "third survives")
	assert.False(t, sessionByRawToken(t, db, fourth.RefreshToken).IsRevoked, "fourth survives")

	assert.Equal(t, http.StatusUnauthorized, doRefresh(t, r, second.RefreshToken).Code)
	assert.Equal(t, http.StatusOK, doRefresh(t, r, third.RefreshToken).Code)
	assert.Equal(t, http.StatusOK, doRefresh(t, r, fourth.RefreshToken).Code)
}

// TestSessionLimit_ScopedByUserAndRole — user ids are polymorphic (client #1 and
// personnel #1 share the numeric id), so neither the count nor the eviction may
// cross roles. Personnel noise must not consume the client budget, and the
// client eviction must leave every personnel row untouched.
func TestSessionLimit_ScopedByUserAndRole(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	reg := registerClientWithEvictions(t, r)

	// Three ACTIVE personnel sessions sharing the client's numeric id.
	var personnelRows []*models.UserSession
	for i, hash := range []string{"hash-limit-personnel-1", "hash-limit-personnel-2", "hash-limit-personnel-3"} {
		row := &models.UserSession{
			Model:            gorm.Model{CreatedAt: time.Now().Add(time.Duration(i) * time.Minute)},
			UserID:           reg.User.ID,
			FamilyID:         "fam-limit-personnel-" + hash,
			RefreshTokenHash: hash,
			ExpiresAt:        time.Now().Add(time.Hour),
			Role:             "personnel",
		}
		assert.NoError(t, db.Create(row).Error)
		personnelRows = append(personnelRows, row)
	}

	// The client budget is untouched by the personnel rows: the second client
	// session still fits.
	second := loginClientWithEvictions(t, r)
	assert.Equal(t, 0, second.SessionsEvicted,
		"another role's sessions must not consume the client budget")

	third := loginClientWithEvictions(t, r)
	assert.Equal(t, 1, third.SessionsEvicted)
	assert.True(t, sessionByRawToken(t, db, reg.RefreshToken).IsRevoked)
	assert.False(t, sessionByRawToken(t, db, second.RefreshToken).IsRevoked)
	assert.False(t, sessionByRawToken(t, db, third.RefreshToken).IsRevoked)
	assert.Equal(t, int64(2), activeSessionCount(t, db, reg.User.ID, "client"))

	// Every personnel session of the same numeric id survives.
	for _, row := range personnelRows {
		var stored models.UserSession
		assert.NoError(t, db.First(&stored, row.ID).Error)
		assert.False(t, stored.IsRevoked, "role %q session %d must not be evicted by a client login", stored.Role, stored.ID)
	}
	assert.Equal(t, int64(3), activeSessionCount(t, db, reg.User.ID, "personnel"))
}

// TestSessionLimit_ExpiredSessionsAreNotActive — an expired-but-unrevoked row
// is lifecycle debris, not a live session: it must neither fill the cap nor be
// revoked by an eviction.
func TestSessionLimit_ExpiredSessionsAreNotActive(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	reg := registerClientWithEvictions(t, r)

	// Two expired rows, older than everything the client is about to create.
	var expired []*models.UserSession
	for i, hash := range []string{"hash-limit-expired-1", "hash-limit-expired-2"} {
		row := &models.UserSession{
			Model:            gorm.Model{CreatedAt: time.Now().Add(-time.Duration(48-i) * time.Hour)},
			UserID:           reg.User.ID,
			FamilyID:         "fam-limit-expired-" + hash,
			RefreshTokenHash: hash,
			ExpiresAt:        time.Now().Add(-time.Hour),
			Role:             "client",
		}
		assert.NoError(t, db.Create(row).Error)
		expired = append(expired, row)
	}

	second := loginClientWithEvictions(t, r)
	assert.Equal(t, 0, second.SessionsEvicted, "expired rows must not count against the cap")
	assert.Equal(t, int64(2), activeSessionCount(t, db, reg.User.ID, "client"))

	// The expired rows stay untouched (not "evicted") and the oldest LIVE
	// session is still alive.
	for _, row := range expired {
		var stored models.UserSession
		assert.NoError(t, db.First(&stored, row.ID).Error)
		assert.False(t, stored.IsRevoked, "expired session %d must not be revoked by the cap", stored.ID)
	}
	assert.False(t, sessionByRawToken(t, db, reg.RefreshToken).IsRevoked)
}

// TestSessionLimit_LogoutAllStillRevokesEverything — the cap must not weaken
// "sign out everywhere": after an eviction, logout-all kills the remaining
// sessions too.
func TestSessionLimit_LogoutAllStillRevokesEverything(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	reg := registerClientWithEvictions(t, r)
	second := loginClientWithEvictions(t, r)
	third := loginClientWithEvictions(t, r)
	assert.Equal(t, 1, third.SessionsEvicted)

	w := do(r, http.MethodPost, "/api/auth-accounts/logout-all", third.Token, "")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Equal(t, http.StatusUnauthorized, doRefresh(t, r, second.RefreshToken).Code)
	assert.Equal(t, http.StatusUnauthorized, doRefresh(t, r, third.RefreshToken).Code)
	assert.Equal(t, int64(0), activeSessionCount(t, db, reg.User.ID, "client"))
}

// TestSessionLimit_EnvOverride — MAX_ACTIVE_SESSIONS replaces the default cap.
func TestSessionLimit_EnvOverride(t *testing.T) {
	t.Setenv("MAX_ACTIVE_SESSIONS", "1")
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	first := registerClientWithEvictions(t, r)
	assert.Equal(t, 0, first.SessionsEvicted)

	second := loginClientWithEvictions(t, r)
	assert.Equal(t, 1, second.SessionsEvicted, "limit 1 means the previous session is evicted")
	assert.Equal(t, int64(1), activeSessionCount(t, db, first.User.ID, "client"))
	assert.True(t, sessionByRawToken(t, db, first.RefreshToken).IsRevoked)
	assert.False(t, sessionByRawToken(t, db, second.RefreshToken).IsRevoked)
}

// TestSessionLimit_InvalidEnvFallsBackToDefault — an unset, non-numeric or
// sub-1 value must fall back to the default of 2 instead of disabling the cap.
func TestSessionLimit_InvalidEnvFallsBackToDefault(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
	}{
		{"non-numeric", "not-a-number"},
		{"zero", "0"},
		{"negative", "-3"},
		{"blank", " "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MAX_ACTIVE_SESSIONS", tc.value)
			relaxAuthRateLimit(t)
			db := setupAuthRefreshDB(t)
			r := authRefreshRouter(t, db)

			first := registerClientWithEvictions(t, r)
			second := loginClientWithEvictions(t, r)
			assert.Equal(t, 0, second.SessionsEvicted, "invalid %q must keep the default cap of 2", tc.value)

			third := loginClientWithEvictions(t, r)
			assert.Equal(t, 1, third.SessionsEvicted, "default cap of 2 must still apply for %q", tc.value)
			assert.True(t, sessionByRawToken(t, db, first.RefreshToken).IsRevoked)
			assert.Equal(t, int64(2), activeSessionCount(t, db, first.User.ID, "client"))
		})
	}
}
