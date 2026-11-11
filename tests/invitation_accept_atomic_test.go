package tests

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInvitationAccept_CreatesAuthAccountAndRejectsSecondAccept — accepting an
// invitation links the staff row AND a local AuthAccount; a second accept is
// rejected and does not duplicate the staff or the account.
func TestInvitationAccept_CreatesAuthAccountAndRejectsSecondAccept(t *testing.T) {
	db := setupAuthRefreshDB(t)
	center := seedDashboardCenter(t, db, 1, "Invite Center", "center-invite@test.com", true)

	invRepo := repository.NewPersonnelInvitationRepositoryImpl(db)
	svc := service.NewPersonnelInvitationServiceImpl(
		invRepo,
		repository.NewPersonnelRepositoryImpl(db),
		repository.NewBeautyCenterRepositoryImpl(db),
		repository.NewClientRepositoryImpl(db),
		repository.NewAuthAccountRepositoryImpl(db),
		validator.New(),
	)

	invitation := models.PersonnelInvitation{
		BeautyCenterID: center.ID,
		Email:          "invited@test.com",
		Token:          "invite-token-123",
		ExpiresAt:      time.Now().Add(time.Hour),
		Status:         "pending",
	}
	require.NoError(t, db.Create(&invitation).Error)

	require.NoError(t, svc.Accept("invite-token-123", "Ines", "Staff", "5550102", "password123"))

	var personnel models.Personnel
	require.NoError(t, db.Where("email = ?", "invited@test.com").First(&personnel).Error)
	assert.True(t, personnel.IsActive)
	assert.Equal(t, center.ID, personnel.BeautyCenterID)

	var account models.AuthAccount
	require.NoError(t, db.Where("provider = ? AND user_type = ? AND personnel_id = ?",
		"local", "personnel", personnel.ID).First(&account).Error)

	// A second accept must fail and change nothing.
	err := svc.Accept("invite-token-123", "Ines", "Staff", "5550102", "password123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no longer valid")

	var personnelCount int64
	require.NoError(t, db.Model(&models.Personnel{}).Where("email = ?", "invited@test.com").Count(&personnelCount).Error)
	assert.Equal(t, int64(1), personnelCount, "a repeated accept must not duplicate the staff row")

	var accountCount int64
	require.NoError(t, db.Model(&models.AuthAccount{}).Where("personnel_id = ?", personnel.ID).Count(&accountCount).Error)
	assert.Equal(t, int64(1), accountCount, "a repeated accept must not duplicate the auth account")

	var reloaded models.PersonnelInvitation
	require.NoError(t, db.Where("token = ?", "invite-token-123").First(&reloaded).Error)
	assert.Equal(t, "accepted", reloaded.Status)
	assert.NotNil(t, reloaded.UsedAt)
}
