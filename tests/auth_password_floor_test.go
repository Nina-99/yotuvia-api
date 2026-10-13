package tests

// Password-floor coverage (min 8): registration and the password-reset flow
// reject a 7-character password and accept an 8-character one. LOGIN is
// deliberately NOT covered by the floor: an existing account whose stored hash
// predates the rule must still be able to sign in.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
)

// TestRegister_PasswordFloor8 — a 7-character registration password is a 400
// validation error; an 8-character one registers successfully (201).
func TestRegister_PasswordFloor8(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	r := authEmailRouter(t, db, utils.NewFakeMailer())

	seven := `{"first_name":"Ana","last_name":"Lopez","email":"floor7@test.com","phone":"5550100","password":"abcdefg","username":"floor7","user_type":"client","terms_accepted":true}`
	w := do(r, http.MethodPost, "/api/auth-accounts/register", "", seven)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "validation failed", errBody(t, w.Body.String()))

	eight := `{"first_name":"Ana","last_name":"Lopez","email":"floor8@test.com","phone":"5550100","password":"abcdefgh","username":"floor8","user_type":"client","terms_accepted":true}`
	w = do(r, http.MethodPost, "/api/auth-accounts/register", "", eight)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

// TestResetPassword_PasswordFloor8 — ResetPassword validates the password
// BEFORE consuming the token, so a 7-character password fails as a validation
// error even with a bogus token, while an 8-character one clears validation and
// only the bogus token is rejected.
func TestResetPassword_PasswordFloor8(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	r := authEmailRouter(t, db, utils.NewFakeMailer())

	w := do(r, http.MethodPost, "/api/auth-accounts/reset-password", "", `{"token":"x","password":"abcdefg"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "validation failed", errBody(t, w.Body.String()))

	w = do(r, http.MethodPost, "/api/auth-accounts/reset-password", "", `{"token":"x","password":"abcdefgh"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "invalid or expired token", errBody(t, w.Body.String()),
		"8 chars must pass validation; the failure must be the token, not the password")
}

// TestResetPassword_Accepts8CharPassword — the full recovery flow with an
// 8-character password: the reset succeeds and the new password logs in.
func TestResetPassword_Accepts8CharPassword(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	mailer := utils.NewFakeMailer()
	r := authEmailRouter(t, db, mailer)

	registerClientAs(t, r, "floor.reset@test.com", "floorreset")

	assert.Equal(t, http.StatusOK,
		do(r, http.MethodPost, "/api/auth-accounts/forgot-password", "", `{"email":"floor.reset@test.com"}`).Code)
	resetToken := tokenFromMessage(t, mustLastMessage(t, mailer))

	resetBody := fmt.Sprintf(`{"token":%q,"password":"abcdefgh"}`, resetToken)
	w := do(r, http.MethodPost, "/api/auth-accounts/reset-password", "", resetBody)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	login := `{"identifier":"floor.reset@test.com","password":"abcdefgh","user_type":"client"}`
	w = do(r, http.MethodPost, "/api/auth-accounts/login", "", login)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// TestLogin_ShortPasswordStillWorks — the login rule was NOT tightened. An
// account seeded with a bcrypt hash of a 6-character password (shorter than the
// new registration floor) still authenticates: the floor only gates password
// WRITES, so no existing user is locked out.
func TestLogin_ShortPasswordStillWorks(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	hash, err := utils.HashPassword("abc123") // 6 characters, below the write floor
	assert.NoError(t, err)
	assert.NoError(t, db.Create(&models.Client{
		FirstName: "Short",
		LastName:  "Pass",
		Username:  "shortpass",
		Email:     "shortpass@test.com",
		Phone:     "555",
		Password:  hash,
		IsActive:  true,
	}).Error)

	body := `{"identifier":"shortpass@test.com","password":"abc123","user_type":"client"}`
	w := do(r, http.MethodPost, "/api/auth-accounts/login", "", body)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp authResponseEnvelope
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Token, "login must issue an access token")
	assert.NotEmpty(t, resp.RefreshToken, "login must issue a refresh token")
}
