package tests

import (
	"strconv"
	"testing"

	"github.com/Nina-99/yottu-api/configs"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/utils"
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
	assert.Len(t, missions, 7, "5 original + 10-reservation + referral")

	counts := map[string]int{}
	for _, m := range missions {
		counts[m.TriggerType+"|"+strconv.Itoa(m.TriggerCount)]++
	}
	for key, n := range counts {
		assert.Equal(t, 1, n, "duplicate seed for %s", key)
	}

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
