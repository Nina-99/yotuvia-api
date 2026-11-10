package tests

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestMissionPeriod_WeeklyResetsAndClaimsOncePerWeek — a weekly mission
// completed and claimed in week W starts again at 0 in W+1, and the new window
// can be completed and claimed independently.
func TestMissionPeriod_WeeklyResetsAndClaimsOncePerWeek(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	sut := newMissionSUT(t)

	mission := &models.TokenMission{
		Name: "Weekly 1", Description: "desc",
		TriggerType: "reservation_completed", TriggerCount: 1,
		Period: "weekly", RewardTokens: 1_000, Icon: "🎯",
		IsActive: true, SortOrder: 1,
	}
	require.NoError(t, sut.db.Create(mission).Error)

	impl := sut.missionSvc.(*service.TokenMissionServiceImpl)
	week45 := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	week46 := week45.AddDate(0, 0, 7)
	impl.Now = func() time.Time { return week45 }

	require.NoError(t, sut.missionSvc.TriggerCheck(1, "reservation_completed"))
	progress := currentMissionProgress(t, sut.missionSvc, mission.ID)
	assert.Equal(t, "2026-W45", progress.PeriodKey)
	assert.Equal(t, 1, progress.CurrentCount)
	assert.NotNil(t, progress.CompletedAt)

	require.NoError(t, sut.missionSvc.ClaimReward(1, mission.ID))
	assert.Equal(t, int64(1_000), accountBalance(t, sut.db))

	// Same week: the claimed row is stable and a second claim conflicts.
	progress = currentMissionProgress(t, sut.missionSvc, mission.ID)
	assert.Equal(t, "2026-W45", progress.PeriodKey)
	assert.True(t, progress.RewardClaimed)
	assert.ErrorIs(t, sut.missionSvc.ClaimReward(1, mission.ID), apperrors.ErrConflict)

	// Next week: fresh row, progress and claim work again.
	impl.Now = func() time.Time { return week46 }
	progress = currentMissionProgress(t, sut.missionSvc, mission.ID)
	assert.Equal(t, "2026-W46", progress.PeriodKey)
	assert.Equal(t, 0, progress.CurrentCount)
	assert.Nil(t, progress.CompletedAt)
	assert.False(t, progress.RewardClaimed)

	require.NoError(t, sut.missionSvc.TriggerCheck(1, "reservation_completed"))
	require.NoError(t, sut.missionSvc.ClaimReward(1, mission.ID))
	assert.Equal(t, int64(2_000), accountBalance(t, sut.db), "the new week pays one more reward")

	// Both window rows are kept as history.
	var rows int64
	require.NoError(t, sut.db.Model(&models.TokenMissionProgress{}).
		Where("beauty_center_id = ? AND mission_id = ?", 1, mission.ID).Count(&rows).Error)
	assert.Equal(t, int64(2), rows)
}

// missionProgress returns the current window row for a mission.
func currentMissionProgress(t *testing.T, svc service.TokenMissionService, missionID uint) models.TokenMissionProgress {
	t.Helper()
	rows, err := svc.GetProgress(1)
	require.NoError(t, err)
	for _, row := range rows {
		if row.MissionID == missionID {
			return row
		}
	}
	t.Fatalf("mission %d not found in progress", missionID)
	return models.TokenMissionProgress{}
}
