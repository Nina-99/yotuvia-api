package tests

// "Sign out everywhere" coverage: POST /api/auth-accounts/logout-all revokes
// every session of the acting account across every family and device —
// including the calling session — and is reachable only behind authentication
// and the shared auth rate limiter.

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/stretchr/testify/assert"
)

// responseMessage extracts the {"message": "..."} success envelope.
func responseMessage(t *testing.T, body string) string {
	t.Helper()
	var parsed struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal([]byte(body), &parsed)
	return parsed.Message
}

// TestLogoutAll_RevokesEverySessionAndRequiresAuth — the actor comes from the
// JWT claims; every session of that account (across families) dies, the caller
// included; a request without a bearer token is 401.
func TestLogoutAll_RevokesEverySessionAndRequiresAuth(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	// Session A (register) and session B (login) — two separate families.
	reg := registerClient(t, r)
	loginBody := `{"identifier":"ana.lopez@test.com","password":"password123","user_type":"client"}`
	w := do(r, http.MethodPost, "/api/auth-accounts/login", "", loginBody)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	_, secondRefresh := parseTokenPair(t, w)
	assert.NotEqual(t, reg.RefreshToken, secondRefresh)

	// A same-numeric-id session of a DIFFERENT role must survive: user_id is
	// polymorphic (client #1 and personnel #1 share the numeric id).
	otherRole := &models.UserSession{
		UserID:           reg.User.ID,
		FamilyID:         "fam-logoutall-personnel",
		RefreshTokenHash: "hash-logoutall-personnel",
		ExpiresAt:        time.Now().Add(time.Hour),
		Role:             "personnel",
	}
	assert.NoError(t, db.Create(otherRole).Error)

	// No bearer token → 401 (the limiter runs first, but a missing token still
	// aborts before the handler).
	w = do(r, http.MethodPost, "/api/auth-accounts/logout-all", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Authenticated caller → 200.
	w = do(r, http.MethodPost, "/api/auth-accounts/logout-all", reg.Token, "")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "signed out of all sessions", responseMessage(t, w.Body.String()))

	// Every session of THIS account is dead, including the calling one.
	assert.Equal(t, http.StatusUnauthorized, doRefresh(t, r, reg.RefreshToken).Code,
		"the calling session must be revoked too")
	assert.Equal(t, http.StatusUnauthorized, doRefresh(t, r, secondRefresh).Code,
		"the other family must be revoked too")

	// The other role's session is untouched.
	var stored models.UserSession
	assert.NoError(t, db.First(&stored, otherRole.ID).Error)
	assert.False(t, stored.IsRevoked, "revocation must not leak to another role with the same numeric id")
}

// TestLogoutAll_BehindSharedAuthRateLimiter — logout-all shares the SAME
// per-IP budget as the rest of the auth surface, so exhausting the budget on
// /login also throttles /logout-all.
func TestLogoutAll_BehindSharedAuthRateLimiter(t *testing.T) {
	t.Setenv("RATE_LIMIT_IP_PER_MIN", "1")
	t.Setenv("RATE_LIMIT_IP_BURST", "2")
	t.Setenv("RATE_LIMIT_IDENTIFIER_PER_MIN", "1000")
	t.Setenv("RATE_LIMIT_IDENTIFIER_BURST", "1000")

	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	// Two requests consume the shared per-IP burst (each still answers 400).
	for i := 0; i < 2; i++ {
		w := do(r, http.MethodPost, "/api/auth-accounts/login", "", "")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}
	// The third request to logout-all is throttled by the same bucket.
	assert429(t, do(r, http.MethodPost, "/api/auth-accounts/logout-all", "", ""))
}
