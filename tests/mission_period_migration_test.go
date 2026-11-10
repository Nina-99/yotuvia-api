package tests

import (
	"testing"

	"github.com/Nina-99/yotuvia-api/configs"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestMigrateMissionPeriods_DropsLegacyIndexAndBackfills — the migration swaps
// the old (center, mission) uniqueness for (center, mission, period_key) and
// backfills legacy rows to "once". It is idempotent.
func TestMigrateMissionPeriods_DropsLegacyIndexAndBackfills(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)

	require.NoError(t, db.Create(&models.BeautyCenter{
		ID: 1, Name: "C", Email: "migrate@test.com", Phone: "1", Password: "x",
	}).Error)
	require.NoError(t, db.Create(&models.TokenMission{
		Name: "Legacy", TriggerType: "reservation_completed", TriggerCount: 5, RewardTokens: 1,
	}).Error)

	progress := &models.TokenMissionProgress{
		BeautyCenterID: 1, MissionID: 1, CurrentCount: 0, TargetCount: 5,
	}
	require.NoError(t, db.Create(progress).Error)
	// Force the legacy empty key the migration must heal.
	require.NoError(t, db.Model(progress).UpdateColumn("period_key", "").Error)

	// Recreate the legacy unique index as it exists on pre-period databases.
	require.NoError(t, db.Exec(
		"CREATE UNIQUE INDEX idx_mission_center ON token_mission_progresses (beauty_center_id, mission_id)",
	).Error)

	configs.MigrateMissionPeriods(db)

	assert.False(t, db.Migrator().HasIndex(&models.TokenMissionProgress{}, "idx_mission_center"))
	assert.True(t, db.Migrator().HasIndex(&models.TokenMissionProgress{}, "idx_mission_center_period"))

	var key string
	require.NoError(t, db.Raw(
		"SELECT period_key FROM token_mission_progresses WHERE beauty_center_id = 1 AND mission_id = 1",
	).Scan(&key).Error)
	assert.Equal(t, "once", key)

	// Idempotent: a second run changes nothing.
	configs.MigrateMissionPeriods(db)
	assert.True(t, db.Migrator().HasIndex(&models.TokenMissionProgress{}, "idx_mission_center_period"))
}
