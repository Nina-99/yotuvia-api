package tests

import (
	"strconv"
	"testing"

	"github.com/Nina-99/yotuvia-api/configs"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestMission_SeedIdempotency runs the seed twice against the same DB and
// verifies the full ladder exists exactly once per definition, with the
// legacy review_responded mission deactivated.
func TestMission_SeedIdempotency(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	configs.DB = db
	defer func() { configs.DB = nil }()

	configs.SeedDefaultMissions()
	configs.SeedDefaultMissions()

	var missions []models.TokenMission
	assert.NoError(t, db.Find(&missions).Error)
	assert.Len(t, missions, 10, "7 lifetime + 3 weekly")

	counts := map[string]int{}
	weeklyRewards := int64(0)
	for _, m := range missions {
		counts[m.TriggerType+"|"+strconv.Itoa(m.TriggerCount)+"|"+m.Period]++
		if m.Period == "weekly" {
			weeklyRewards += m.RewardTokens
		}
	}
	for key, n := range counts {
		assert.Equal(t, 1, n, "duplicate seed for %s", key)
	}
	assert.Equal(t, int64(6000), weeklyRewards, "weekly set must pay 6 Ytv max")

	var legacyActive int64
	assert.NoError(t, db.Model(&models.TokenMission{}).
		Where("trigger_type = ? AND is_active = ?", "review_responded", true).
		Count(&legacyActive).Error)
	assert.Zero(t, legacyActive, "legacy review_responded deactivated by D3")

	var referralActive int64
	assert.NoError(t, db.Model(&models.TokenMission{}).
		Where("trigger_type = ? AND is_active = ?", "referral", true).
		Count(&referralActive).Error)
	assert.Equal(t, int64(1), referralActive)
}
