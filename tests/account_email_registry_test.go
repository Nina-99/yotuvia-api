package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAccountEmailRegistry_ReserveIsUnique — the unique index is the race-proof
// guarantee: a second reservation for the same email fails even when it targets
// a different role.
func TestAccountEmailRegistry_ReserveIsUnique(t *testing.T) {
	db := setupAuthRefreshDB(t)
	repo := repository.NewAccountEmailRepositoryImpl(db)

	entry, err := repo.Reserve(context.Background(), "same@test.com", "client")
	require.NoError(t, err)
	require.NotNil(t, entry)

	_, err = repo.Reserve(context.Background(), "same@test.com", "beauty_center")
	assert.ErrorIs(t, err, repository.ErrEmailAlreadyReserved)

	require.NoError(t, repo.BindUser(context.Background(), entry.ID, 7))
	var stored models.AccountEmail
	require.NoError(t, db.Where("email = ?", "same@test.com").First(&stored).Error)
	assert.Equal(t, uint(7), stored.UserID)
	assert.Equal(t, "client", stored.UserType)

	// Releasing a failed reservation frees the email.
	require.NoError(t, repo.Release(context.Background(), entry.ID))
	_, err = repo.Reserve(context.Background(), "same@test.com", "beauty_center")
	require.NoError(t, err)
}

// TestRegister_PopulatesEmailRegistry — a successful registration binds the
// reservation to the new client row.
func TestRegister_PopulatesEmailRegistry(t *testing.T) {
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	resp := registerClient(t, r)

	var entry models.AccountEmail
	require.NoError(t, db.Where("email = ?", "ana.lopez@test.com").First(&entry).Error)
	assert.Equal(t, "client", entry.UserType)
	assert.Equal(t, resp.User.ID, entry.UserID)
}

// TestCompleteRegistration_PopulatesEmailRegistry — the passwordless path also
// claims the email before creating the account.
func TestCompleteRegistration_PopulatesEmailRegistry(t *testing.T) {
	setHighLimits(t)
	db := setupAuthRefreshDB(t)
	r, mailer := otpRouter(t, db)

	doRequestCode(t, r, "registry-new@test.com")
	res := verifyCode(t, r, "registry-new@test.com", lastCode(t, mailer, "registry-new@test.com"))
	require.Equal(t, 200, res.code, res.body)

	var payload struct {
		RegistrationTicket string `json:"registration_ticket"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.body), &payload))

	body := `{"registration_ticket":"` + payload.RegistrationTicket + `","user_type":"client","first_name":"Ana","last_name":"Lopez","phone":"5550100","terms_accepted":true}`
	w := do(r, http.MethodPost, "/api/auth-accounts/register/complete", "", body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var entry models.AccountEmail
	require.NoError(t, db.Where("email = ?", "registry-new@test.com").First(&entry).Error)
	assert.Equal(t, "client", entry.UserType)
	assert.NotZero(t, entry.UserID)
}
