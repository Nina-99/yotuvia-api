package configs

import (
	"context"

	"github.com/Nina-99/yotuvia-api/utils"
	"gorm.io/gorm"
)

// schedulerAdvisoryLockKey is the stable Postgres advisory-lock key that
// arbitrates a scheduler tick across API replicas. 0x594F545455 is ASCII
// "YOTTU" — readable and collision-safe. Every replica MUST use this same key
// for the lock to serialize ticks.
const schedulerAdvisoryLockKey int64 = 0x594F545455

// acquireTickLock tries to take the cluster-wide scheduler advisory lock.
//
// Postgres advisory locks are SESSION-scoped, so acquire and release must run
// on the SAME database connection: with GORM's pool, releasing on a different
// pooled connection would silently leave the lock held until that session ends.
// We therefore pin one dedicated *sql.Conn for the lifetime of the tick and
// unlock on it before returning it to the pool.
//
// Dialect safety: the test suite (and any SQLite deployment) has no pg_*
// functions, so on any dialect other than Postgres this is a clean no-op that
// reports "acquired" and lets the tick run unchanged. This is deliberate and
// MUST stay a no-op so the suite never depends on a Postgres server.
//
// acquired=false means another replica holds the lock and this replica must
// skip the tick. The returned release is always safe to call and should be
// deferred so a panic inside a sweep still unlocks.
func acquireTickLock(db *gorm.DB) (release func(), acquired bool) {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return func() {}, true
	}

	sqlDB, err := db.DB()
	if err != nil {
		utils.Log.Errorf("[SCHEDULER] advisory lock: cannot reach sql.DB: %v", err)
		return func() {}, false
	}

	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		utils.Log.Errorf("[SCHEDULER] advisory lock: cannot pin connection: %v", err)
		return func() {}, false
	}

	var got bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", schedulerAdvisoryLockKey).Scan(&got); err != nil {
		utils.Log.Errorf("[SCHEDULER] advisory lock: acquire failed: %v", err)
		_ = conn.Close()
		return func() {}, false
	}
	if !got {
		// Normal multi-replica case: another replica owns this tick.
		_ = conn.Close()
		return func() {}, false
	}

	return func() {
		// Best-effort unlock: closing the connection also drops a session lock.
		_, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", schedulerAdvisoryLockKey)
		_ = conn.Close()
	}, true
}
