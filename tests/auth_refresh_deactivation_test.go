package tests

import (
	"net/http"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRefresh_RejectsDeactivatedClient — a refresh token minted while the
// account was active must stop working the moment the account is deactivated,
// and every session of that account is revoked.
func TestRefresh_RejectsDeactivatedClient(t *testing.T) {
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	resp := registerClient(t, r)
	require.NoError(t, db.Model(&models.Client{}).
		Where("email = ?", "ana.lopez@test.com").
		Update("is_active", false).Error)

	w := doRefresh(t, r, resp.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	assert.Equal(t, "unauthorized", errBody(t, w.Body.String()))

	var live int64
	require.NoError(t, db.Model(&models.UserSession{}).
		Where("user_id = ? AND role = ? AND is_revoked = ?", resp.User.ID, "client", false).
		Count(&live).Error)
	assert.Zero(t, live, "deactivation must revoke every live session")
}

// TestClientDeactivation_RevokesSessions — the client is deactivated through the
// update path (is_active:false), so older access/refresh tokens die immediately.
func TestClientDeactivation_RevokesSessions(t *testing.T) {
	db := setupAuthRefreshDB(t)
	resp := registerClient(t, authRefreshRouter(t, db))

	clientRepo := repository.NewClientRepositoryImpl(db)
	sessionRepo := repository.NewUserSessionRepositoryImpl(db)
	svc := service.NewClientServiceImpl(clientRepo, sessionRepo, validator.New())

	client, err := clientRepo.FindById(resp.User.ID)
	require.NoError(t, err)
	require.NotNil(t, client)

	inactive := false
	err = svc.Update(client.ID, client.ID, request.UpdateClientRequest{
		FirstName: client.FirstName,
		LastName:  client.LastName,
		Username:  client.Username,
		Email:     client.Email,
		Phone:     client.Phone,
		IsActive:  &inactive,
	})
	require.NoError(t, err)

	var live int64
	require.NoError(t, db.Model(&models.UserSession{}).
		Where("user_id = ? AND role = ? AND is_revoked = ?", client.ID, "client", false).
		Count(&live).Error)
	assert.Zero(t, live, "deactivating a client must revoke its sessions")
}

// TestPersonnelDeactivation_RevokesSessions — same guarantee for staff managed
// by the centre owner.
func TestPersonnelDeactivation_RevokesSessions(t *testing.T) {
	db := setupAuthRefreshDB(t)

	personnel := models.Personnel{
		BeautyCenterID: 1,
		FirstName:      "Ana",
		LastName:       "Staff",
		Email:          "staff-deact@test.com",
		Phone:          "5550100",
		Password:       "irrelevant",
		IsActive:       true,
	}
	require.NoError(t, db.Create(&personnel).Error)
	require.NoError(t, db.Create(&models.UserSession{
		UserID:           personnel.ID,
		Role:             "personnel",
		FamilyID:         "fam-staff",
		RefreshTokenHash: "hash-staff",
		ExpiresAt:        time.Now().Add(time.Hour),
	}).Error)

	svc := service.NewPersonnelServiceImpl(
		repository.NewPersonnelRepositoryImpl(db),
		repository.NewServiceRepositoryImpl(db),
		repository.NewUserSessionRepositoryImpl(db),
		validator.New(),
	)
	_, err := svc.Update(personnel.ID, personnel.BeautyCenterID, request.UpdatePersonnelRequest{IsActive: false})
	require.NoError(t, err)

	var live int64
	require.NoError(t, db.Model(&models.UserSession{}).
		Where("user_id = ? AND role = ? AND is_revoked = ?", personnel.ID, "personnel", false).
		Count(&live).Error)
	assert.Zero(t, live, "deactivating staff must revoke its sessions")
}
