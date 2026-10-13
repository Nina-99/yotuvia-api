package configs

import (
	"errors"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	return db
}

// TestAcquireTickLock_NoOpOnNonPostgres pins the dialect safety contract: the
// whole suite runs on SQLite, where pg_* functions do not exist. The helper
// must report "acquired" and its release must be safe so a tick still runs.
//
// NOTE: this does NOT exercise the real Postgres advisory lock — that requires
// a Postgres server the test environment does not provide.
func TestAcquireTickLock_NoOpOnNonPostgres(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := newSQLiteDB(t)

	release, acquired := acquireTickLock(db)
	assert.True(t, acquired, "non-postgres dialect must be a no-op that lets the tick run")
	assert.NotPanics(t, release, "release must be safe to call")
}

// TestRunSchedulerTick_RunsAllFourSweeps guards the loop's contract: one tick
// must drive reminders, no-shows, holds and waitlist offers, in that order.
func TestRunSchedulerTick_RunsAllFourSweeps(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()

	var calls []string
	runSchedulerTick(schedulerSweeps{
		sendDueReminders: func(time.Time) (int, error) { calls = append(calls, "reminders"); return 0, nil },
		sweepNoShows:     func(time.Time, int) (int, error) { calls = append(calls, "no-shows"); return 0, nil },
		expireHolds:      func(time.Time) (int, error) { calls = append(calls, "holds"); return 0, nil },
		sweepOffers:      func(time.Time) (int, error) { calls = append(calls, "offers"); return 0, nil },
	}, time.Now())

	assert.Equal(t, []string{"reminders", "no-shows", "holds", "offers"}, calls)
}

// TestRunSchedulerTick_FailingSweepDoesNotStopTheRest is the best-effort
// contract: a failing sweep is logged, never fatal, and the remaining sweeps
// still run. This is what keeps a single bad query from starving the loop.
func TestRunSchedulerTick_FailingSweepDoesNotStopTheRest(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()

	var noShowRan, holdsRan, offersRan bool
	runSchedulerTick(schedulerSweeps{
		sendDueReminders: func(time.Time) (int, error) { return 0, errors.New("reminders exploded") },
		sweepNoShows:     func(time.Time, int) (int, error) { noShowRan = true; return 0, errors.New("boom") },
		expireHolds:      func(time.Time) (int, error) { holdsRan = true; return 0, nil },
		sweepOffers:      func(time.Time) (int, error) { offersRan = true; return 0, nil },
	}, time.Now())

	assert.True(t, noShowRan, "no-show sweep must still run after a failed reminder sweep")
	assert.True(t, holdsRan, "hold sweep must still run after a failed no-show sweep")
	assert.True(t, offersRan, "offer sweep must still run after a failed hold sweep")
}

// TestStartBackgroundScheduler_Disabled — SCHEDULER_ENABLED=false returns
// immediately without building services or starting a goroutine.
func TestStartBackgroundScheduler_Disabled(t *testing.T) {
	t.Setenv("SCHEDULER_ENABLED", "false")
	utils.Log = zap.NewNop().Sugar()
	db := newSQLiteDB(t)

	assert.NotPanics(t, func() { StartBackgroundScheduler(db) })
}
