package tests

// Coverage for the registration-terms work:
//   PART A — terms acceptance at registration (400 without, server-stamped
//   timestamp + version with).
//   PART B — duplicate email/username answer 409 with a stable, distinct
//   message instead of the old 500.
//   PART C — the /auth-accounts/check-email enumeration oracle is gone (404).

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/stretchr/testify/assert"
)

// clientRegisterBody builds a client registration payload with an explicit
// terms flag, so a test can exercise both the accepted and rejected shapes.
func clientRegisterBody(email, username string, terms bool) string {
	return fmt.Sprintf(
		`{"first_name":"Ana","last_name":"Lopez","email":%q,"phone":"5550100","password":"password123","username":%q,"user_type":"client","terms_accepted":%t}`,
		email, username, terms)
}

// TestRegister_RequiresTermsAcceptance — a registration that does not assert
// terms acceptance is rejected with 400 and creates nothing.
func TestRegister_RequiresTermsAcceptance(t *testing.T) {
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	w := do(r, http.MethodPost, "/api/auth-accounts/register", "", clientRegisterBody("no.terms@test.com", "noterms", false))
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	var count int64
	db.Model(&models.Client{}).Count(&count)
	assert.Equal(t, int64(0), count, "a rejected registration must not create a client")

	// Omitting the flag entirely behaves the same (bool zero value).
	body := `{"first_name":"Ana","last_name":"Lopez","email":"missing.terms@test.com","phone":"5550100","password":"password123","username":"missingterms","user_type":"client"}`
	w = do(r, http.MethodPost, "/api/auth-accounts/register", "", body)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestRegister_RecordsServerStampedTerms — the server stamps the acceptance
// timestamp and the SERVER version (TERMS_VERSION), never a client value.
func TestRegister_RecordsServerStampedTerms(t *testing.T) {
	t.Setenv("TERMS_VERSION", "2026-09")
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	w := do(r, http.MethodPost, "/api/auth-accounts/register", "", clientRegisterBody("terms@test.com", "terms", true))
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var resp struct {
		User struct {
			ID uint `json:"id"`
		} `json:"user"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	var acct models.AuthAccount
	err := db.Where("provider = ? AND client_id = ?", "local", resp.User.ID).First(&acct).Error
	assert.NoError(t, err, "registration must persist an acceptance record")
	assert.NotNil(t, acct.TermsAcceptedAt, "the acceptance timestamp must be stamped")
	assert.Equal(t, "2026-09", acct.TermsVersion, "the server-authoritative version must be stored")
}

// TestRegister_TermsVersionDefaults — with TERMS_VERSION unset the explicit
// default "1" is stamped.
func TestRegister_TermsVersionDefaults(t *testing.T) {
	t.Setenv("TERMS_VERSION", "")
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	w := do(r, http.MethodPost, "/api/auth-accounts/register", "", clientRegisterBody("defaultversion@test.com", "defaultversion", true))
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var acct models.AuthAccount
	err := db.Where("provider = ? AND user_type = ?", "local", "client").First(&acct).Error
	assert.NoError(t, err)
	assert.Equal(t, "1", acct.TermsVersion)
}

// TestRegister_BeautyCenterTermsRecorded — the beauty-center registration path
// records the acceptance too.
func TestRegister_BeautyCenterTermsRecorded(t *testing.T) {
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	body := `{"first_name":"Studio","last_name":"","email":"center.terms@test.com","phone":"5550100","password":"password123","user_type":"beauty_center","address":"123 Main","terms_accepted":true}`
	w := do(r, http.MethodPost, "/api/auth-accounts/register", "", body)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var acct models.AuthAccount
	err := db.Where("provider = ? AND user_type = ?", "local", "beauty_center").First(&acct).Error
	assert.NoError(t, err, "beauty center registration must persist an acceptance record")
	assert.NotNil(t, acct.TermsAcceptedAt)
	assert.Equal(t, "1", acct.TermsVersion)
	assert.NotNil(t, acct.BeautyCenterID)
}

// TestRegister_PersonnelSelfRegistrationTermsRecorded — the public register
// endpoint's personnel branch is a real self-registration surface, so it also
// records the acceptance. (The admin POST /auth/register-personnel path does
// NOT — see RegisterPersonnelDecision in the report.)
func TestRegister_PersonnelSelfRegistrationTermsRecorded(t *testing.T) {
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	body := `{"first_name":"Pam","last_name":"Personnel","email":"staff.terms@test.com","phone":"5550100","password":"password123","user_type":"personnel","terms_accepted":true}`
	w := do(r, http.MethodPost, "/api/auth-accounts/register", "", body)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var acct models.AuthAccount
	err := db.Where("provider = ? AND user_type = ?", "local", "personnel").First(&acct).Error
	assert.NoError(t, err, "personnel self-registration must persist an acceptance record")
	assert.NotNil(t, acct.TermsAcceptedAt)
	assert.NotNil(t, acct.PersonnelID)
}

// TestRegister_DuplicateEmail409 — the old 500 collapses to a 409 with the
// stable email message (PART B).
func TestRegister_DuplicateEmail409(t *testing.T) {
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	assert.Equal(t, http.StatusCreated,
		do(r, http.MethodPost, "/api/auth-accounts/register", "", clientRegisterBody("dup@test.com", "dup-one", true)).Code)

	w := do(r, http.MethodPost, "/api/auth-accounts/register", "", clientRegisterBody("dup@test.com", "dup-two", true))
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, "email already exists", errBody(t, w.Body.String()))
}

// TestRegister_DuplicateUsername409 — the username collision is distinct from
// the email one: 409 plus its own stable message.
func TestRegister_DuplicateUsername409(t *testing.T) {
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	assert.Equal(t, http.StatusCreated,
		do(r, http.MethodPost, "/api/auth-accounts/register", "", clientRegisterBody("first@test.com", "shared-name", true)).Code)

	w := do(r, http.MethodPost, "/api/auth-accounts/register", "", clientRegisterBody("second@test.com", "shared-name", true))
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, "username already exists", errBody(t, w.Body.String()))
	assert.NotEqual(t, "email already exists", errBody(t, w.Body.String()), "the two messages must stay distinguishable")
}

// TestCheckEmailRouteRemoved — the enumeration oracle is gone: the route no
// longer exists (PART C).
func TestCheckEmailRouteRemoved(t *testing.T) {
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	w := do(r, http.MethodPost, "/api/auth-accounts/check-email", "", `{"email":"anyone@test.com"}`)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestMapToHTTP_TakenSentinels — both new sentinels map to 409 with their own
// stable text.
func TestMapToHTTP_TakenSentinels(t *testing.T) {
	status, msg := apperrors.MapToHTTP(apperrors.ErrEmailTaken)
	assert.Equal(t, http.StatusConflict, status)
	assert.Equal(t, "email already exists", msg)

	status, msg = apperrors.MapToHTTP(apperrors.ErrUsernameTaken)
	assert.Equal(t, http.StatusConflict, status)
	assert.Equal(t, "username already exists", msg)
}
