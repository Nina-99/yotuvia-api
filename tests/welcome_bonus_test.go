package tests

import (
	"testing"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestMarkVerified_RequiresCompleteProfile — a fake/empty center cannot be
// verified before filling its information.
func TestMarkVerified_RequiresCompleteProfile(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedDashboardCenter(t, db, 1, "Empty Center", "empty@test.com", false)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)

	svc := service.NewBeautyCenterServiceImpl(
		repository.NewBeautyCenterRepositoryImpl(db), nil, validator.New(), nil,
		service.BeautyCenterDeps{TokenAccountRepo: accountRepo},
	)

	err := svc.MarkVerified(1)
	assert.ErrorIs(t, err, apperrors.ErrValidation)

	var center models.BeautyCenter
	require.NoError(t, db.First(&center, 1).Error)
	assert.Nil(t, center.VerifiedAt)
	assert.False(t, center.WelcomeBonusPaid)
}

// TestMarkVerified_PaysWelcomeBonusOnce — the 25 Ytv welcome bonus is paid at
// verification, exactly once, even if verification is retried.
func TestMarkVerified_PaysWelcomeBonusOnce(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedDashboardCenter(t, db, 1, "Complete Center", "complete@test.com", false)
	completeCenterProfile(t, db, 1)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)

	svc := service.NewBeautyCenterServiceImpl(
		repository.NewBeautyCenterRepositoryImpl(db), nil, validator.New(), nil,
		service.BeautyCenterDeps{TokenAccountRepo: accountRepo},
	)

	require.NoError(t, svc.MarkVerified(1))
	assert.Equal(t, int64(25_000), balanceOf(t, accountRepo, 1))

	require.NoError(t, svc.MarkVerified(1), "re-verification is a no-op")
	assert.Equal(t, int64(25_000), balanceOf(t, accountRepo, 1), "welcome bonus must not pay twice")

	var center models.BeautyCenter
	require.NoError(t, db.First(&center, 1).Error)
	assert.NotNil(t, center.VerifiedAt)
	assert.True(t, center.WelcomeBonusPaid)
}
