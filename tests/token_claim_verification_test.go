package tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestTokenClaim_RequiresVerifiedCenter — unbacked mission rewards are gated on
// platform verification: an unverified center gets 409, a verified one claims.
func TestTokenClaim_RequiresVerifiedCenter(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := authzTestRouter(t)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)

	seedDashboardCenter(t, db, 1, "Claim Center", "claim@test.com", false)
	require.NoError(t, db.Create(&models.TokenAccount{
		BeautyCenterID: 1, Balance: 0, Currency: "BOB",
	}).Error)

	now := time.Now().UTC()
	mission := &models.TokenMission{
		Name: "One", TriggerType: "reservation_completed",
		TriggerCount: 1, Period: "once", RewardTokens: 1_000, IsActive: true,
	}
	require.NoError(t, db.Create(mission).Error)
	require.NoError(t, db.Create(&models.TokenMissionProgress{
		BeautyCenterID: 1, MissionID: mission.ID, PeriodKey: "once",
		CurrentCount: 1, TargetCount: 1, CompletedAt: &now,
	}).Error)

	token := roleToken(t, 1, 1, "beauty_center")
	claimPath := fmt.Sprintf("/api/tokens/missions/%d/claim", mission.ID)

	w := do(r, http.MethodPost, claimPath, token, "")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())

	require.NoError(t, db.Model(&models.BeautyCenter{}).Where("id = ?", 1).
		Update("verified_at", now).Error)

	w = do(r, http.MethodPost, claimPath, token, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, int64(1_000), balanceOf(t, accountRepo, 1))
}
