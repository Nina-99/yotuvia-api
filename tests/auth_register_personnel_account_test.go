package tests

import (
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisterPersonnel_CreatesLocalAuthAccount — staff created by the owner get
// a local AuthAccount, so password recovery can issue a reset token for them.
func TestRegisterPersonnel_CreatesLocalAuthAccount(t *testing.T) {
	db := setupAuthRefreshDB(t)
	seedDashboardCenter(t, db, 1, "Owner Center", "owner-reset@test.com", true)
	r := authRefreshRouter(t, db)

	owner := roleToken(t, 1, 1, "beauty_center")
	body := `{"first_name":"Ana","last_name":"Staff","email":"staff-reset@test.com","phone":"5550100","password":"password123"}`
	w := do(r, http.MethodPost, "/api/auth/register-personnel", owner, body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var account models.AuthAccount
	require.NoError(t, db.Where("provider = ? AND user_type = ? AND provider_id = ?",
		"local", "personnel", "staff-reset@test.com").First(&account).Error)
	require.NotNil(t, account.PersonnelID)
	assert.Nil(t, account.TermsAcceptedAt, "owner-created staff did not accept terms")

	// Forgot-password must mint a reset token bound to that account.
	w = do(r, http.MethodPost, "/api/auth-accounts/forgot-password", "",
		`{"email":"staff-reset@test.com"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var tokens []models.AuthToken
	require.NoError(t, db.Where("user_type = ? AND user_id = ? AND purpose = ?",
		"personnel", *account.PersonnelID, models.TokenPurposePasswordReset).Find(&tokens).Error)
	assert.NotEmpty(t, tokens, "staff with a local account must be able to reset their password")
}
