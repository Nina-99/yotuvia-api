package tests

import (
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestAuthEmailUniqueness_ClientThenCenter — one email belongs to one account:
// a client email cannot register a centre.
func TestAuthEmailUniqueness_ClientThenCenter(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	w := do(r, http.MethodPost, "/api/auth-accounts/register", "",
		clientRegisterBody("same@test.com", "sameuser", true))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	center := `{"first_name":"Same Center","email":"same@test.com","phone":"5550101","password":"password123","user_type":"beauty_center","terms_accepted":true}`
	w = do(r, http.MethodPost, "/api/auth-accounts/register", "", center)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, "email already exists", errBody(t, w.Body.String()))
}

// TestAuthEmailUniqueness_RegisterPersonnel — a client email cannot be reused
// as staff either.
func TestAuthEmailUniqueness_RegisterPersonnel(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	t.Setenv("JWT_SECRET", testJWTSecret)
	t.Setenv("RATE_LIMIT_IP_PER_MIN", "10000")
	t.Setenv("RATE_LIMIT_IP_BURST", "10000")

	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)
	seedDashboardCenter(t, db, 1, "Owner Center", "owner-unique@test.com", true)

	w := do(r, http.MethodPost, "/api/auth-accounts/register", "",
		clientRegisterBody("staff-same@test.com", "staffsame", true))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	owner := roleToken(t, 1, 1, "beauty_center")
	body := `{"first_name":"Staff","last_name":"Same","email":"staff-same@test.com","phone":"5550102","password":"password123"}`
	w = do(r, http.MethodPost, "/api/auth/register-personnel", owner, body)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())

	// A fresh email still works.
	body = `{"first_name":"Staff","last_name":"Fresh","email":"staff-fresh@test.com","phone":"5550103","password":"password123"}`
	w = do(r, http.MethodPost, "/api/auth/register-personnel", owner, body)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}
