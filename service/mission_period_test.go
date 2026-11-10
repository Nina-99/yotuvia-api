package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMissionPeriodKey(t *testing.T) {
	// 2026-11-02 is the Monday that starts ISO week 45.
	mondayW45 := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	sundayW45 := time.Date(2026, 11, 8, 23, 59, 0, 0, time.UTC)
	mondayW46 := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)

	assert.Equal(t, "once", MissionPeriodKey("", mondayW45), "legacy/empty period behaves as once")
	assert.Equal(t, "once", MissionPeriodKey("once", mondayW45))
	assert.Equal(t, "2026-W45", MissionPeriodKey("weekly", mondayW45))
	assert.Equal(t, "2026-W45", MissionPeriodKey("weekly", sundayW45), "Sunday still belongs to its ISO week")
	assert.Equal(t, "2026-W46", MissionPeriodKey("weekly", mondayW46), "Monday rolls the week")
	assert.Equal(t, "2026-11", MissionPeriodKey("monthly", mondayW45))
}

func TestMissionPeriodKey_UsesUTC(t *testing.T) {
	// 2026-11-09 00:30 at UTC-4 is still 2026-11-09 04:30 UTC → W46.
	loc := time.FixedZone("BOT", -4*60*60)
	local := time.Date(2026, 11, 8, 23, 30, 0, 0, loc) // 03:30 UTC on Nov 9
	assert.Equal(t, "2026-W46", MissionPeriodKey("weekly", local))
}
