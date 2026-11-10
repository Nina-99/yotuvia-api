package tests

import (
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestPlatformVerifyCenter_CreditsReferralOnce — the platform verification
// endpoint is the missing trigger: it stamps verified_at, credits the pending
// referral atomically and is idempotent.
func TestPlatformVerifyCenter_CreditsReferralOnce(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := platformTestRouter(t)
	seedPlatformFixtures(t, db)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)

	require.NoError(t, db.Create(&models.TokenAccount{BeautyCenterID: 1, Balance: 0, Currency: "BOB"}).Error)
	require.NoError(t, db.Create(&models.TokenAccount{BeautyCenterID: 2, Balance: 0, Currency: "BOB"}).Error)
	completeCenterProfile(t, db, 2)
	require.NoError(t, db.Create(&models.Referral{
		ReferrerID: 1, ReferredID: 2, Status: "pending",
		RewardReferrer: 20_000, RewardReferred: 10_000,
	}).Error)
	activateReferralCenter(t, db, 2)

	token := roleToken(t, 900, 0, "super_admin")
	w := do(r, http.MethodPut, "/api/admin/platform/centers/2/verify", token, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Equal(t, int64(20_000), balanceOf(t, accountRepo, 1))
	// Referred center: 10 Ytv referral + 25 Ytv one-time welcome bonus.
	assert.Equal(t, int64(35_000), balanceOf(t, accountRepo, 2))

	// Idempotent: a second verification answers 200 and moves nothing.
	w = do(r, http.MethodPut, "/api/admin/platform/centers/2/verify", token, "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(20_000), balanceOf(t, accountRepo, 1))
	assert.Equal(t, int64(35_000), balanceOf(t, accountRepo, 2))

	var referral models.Referral
	require.NoError(t, db.First(&referral).Error)
	assert.Equal(t, "credited", referral.Status)
}
