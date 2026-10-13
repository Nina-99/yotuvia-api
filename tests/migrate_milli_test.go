package tests

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/Nina-99/yotuvia-api/migration"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMigrationDB builds an in-memory SQLite DB shaped like the legacy schema:
// the 5 ledger tables with float (REAL) money columns, exactly the 11 fields
// the migration converts. The migration engine is dialect-agnostic (Postgres
// in production via scripts/migrate_milli.go, SQLite here) so the same
// Upgrade/Downgrade code path runs in CI without a Postgres server.
func newMigrationDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	// mattn/go-sqlite3 gives each pooled connection its own private :memory:
	// DB; pin to one connection so DDL/DML share the same database.
	db.SetMaxOpenConns(1)

	stmts := []string{
		`CREATE TABLE token_accounts (
			id INTEGER PRIMARY KEY, beauty_center_id INTEGER,
			balance REAL, total_purchased REAL, total_consumed REAL, currency TEXT)`,
		`CREATE TABLE token_transactions (
			id INTEGER PRIMARY KEY, amount REAL, balance_before REAL, balance_after REAL,
			type TEXT, reference_id INTEGER)`,
		`CREATE TABLE token_top_ups (
			id INTEGER PRIMARY KEY, tokens_received REAL, bonus_tokens REAL, amount_bs REAL, status TEXT)`,
		`CREATE TABLE token_missions (
			id INTEGER PRIMARY KEY, reward_tokens REAL, name TEXT)`,
		`CREATE TABLE referrals (
			id INTEGER PRIMARY KEY, reward_referrer REAL, reward_referred REAL, status TEXT)`,
	}
	for _, s := range stmts {
		_, err := db.Exec(s)
		require.NoError(t, err, "setup: %s", s)
	}
	return db
}

func seedLegacyRows(t *testing.T, db *sql.DB) {
	t.Helper()
	stmts := []string{
		`INSERT INTO token_accounts (id, beauty_center_id, balance, total_purchased, total_consumed, currency)
		 VALUES (1, 1, 25.50, 10.00, 8.25, 'USD'), (2, 2, 25.5555, 0, 0, 'USD')`,
		`INSERT INTO token_transactions (id, amount, balance_before, balance_after, type, reference_id)
		 VALUES (1, -3.25, 25.50, 22.25, 'consumption', 1)`,
		`INSERT INTO token_top_ups (id, tokens_received, bonus_tokens, amount_bs, status)
		 VALUES (1, 25.50, 2.00, 50.00, 'approved')`,
		`INSERT INTO token_missions (id, reward_tokens, name)
		 VALUES (1, 5.00, 'first_visit')`,
		`INSERT INTO referrals (id, reward_referrer, reward_referred, status)
		 VALUES (1, 20.00, 10.00, 'credited')`,
	}
	for _, s := range stmts {
		_, err := db.Exec(s)
		require.NoError(t, err, "seed: %s", s)
	}
}

// TestMigrateMilli_BackfillValues — pure backfill math contract (spec:
// "Two-decimal legacy value converts exactly" 25.50→25500 and "Sub-milli
// legacy value rounds half-up" 25.5555→25556). The migration MUST use the
// same canonical rounding as utils.ToMilli so stored milli values always
// agree with what the app writes.
func TestMigrateMilli_BackfillValues(t *testing.T) {
	assert.Equal(t, int64(25500), migration.BackfillMilli(25.50))
	assert.Equal(t, int64(25556), migration.BackfillMilli(25.5555))
	assert.Equal(t, int64(-25556), migration.BackfillMilli(-25.5555))
	assert.Equal(t, int64(100), migration.BackfillMilli(0.10))
}

// TestMigrateMilli_UpgradeBackfillsExactly — end-to-end over SQLite: after
// Upgrade every legacy float becomes exact milli (25500, 25556), the float
// column is dropped and the *_milli column renamed in place.
func TestMigrateMilli_UpgradeBackfillsExactly(t *testing.T) {
	db := newMigrationDB(t)
	defer db.Close()
	seedLegacyRows(t, db)

	rep, err := migration.Upgrade(context.Background(), db, migration.DialectSQLite)
	require.NoError(t, err)
	require.Equal(t, 0, rep.Mismatches)

	// Column exists after rename: read balance as int64 milli.
	var bal, bal2 int64
	require.NoError(t, db.QueryRow(`SELECT balance FROM token_accounts WHERE id = 1`).Scan(&bal))
	require.NoError(t, db.QueryRow(`SELECT balance FROM token_accounts WHERE id = 2`).Scan(&bal2))
	assert.Equal(t, int64(25500), bal)  // 25.50 -> 25500
	assert.Equal(t, int64(25556), bal2) // 25.5555 -> 25556 (half-up)

	// Float column is gone; the milli column was renamed to the original name.
	var typeName string
	require.NoError(t, db.QueryRow(`SELECT type FROM pragma_table_info('token_accounts') WHERE name = 'balance'`).Scan(&typeName))
	assert.Equal(t, "BIGINT", typeName)

	// Full check of the other converted columns.
	var txnAmount, topupReceived, missionReward, refReferrer int64
	require.NoError(t, db.QueryRow(`SELECT amount FROM token_transactions WHERE id = 1`).Scan(&txnAmount))
	require.NoError(t, db.QueryRow(`SELECT tokens_received FROM token_top_ups WHERE id = 1`).Scan(&topupReceived))
	require.NoError(t, db.QueryRow(`SELECT reward_tokens FROM token_missions WHERE id = 1`).Scan(&missionReward))
	require.NoError(t, db.QueryRow(`SELECT reward_referrer FROM referrals WHERE id = 1`).Scan(&refReferrer))
	assert.Equal(t, int64(-3250), txnAmount) // -3.25 -> -3250
	assert.Equal(t, int64(25500), topupReceived)
	assert.Equal(t, int64(5000), missionReward) // 5.00 -> 5000
	assert.Equal(t, int64(20000), refReferrer)  // 20.00 -> 20000
}

// TestMigrateMilli_VerifyMismatchBlocksCutover — a corrupted milli value (or a
// row that was never backfilled) MUST abort the migration and retain the float
// columns: data is never silently dropped (spec: "Verification mismatch blocks
// float removal").
func TestMigrateMilli_VerifyMismatchBlocksCutover(t *testing.T) {
	db := newMigrationDB(t)
	defer db.Close()
	seedLegacyRows(t, db)

	// Simulate a partial/prior run that left a WRONG milli value behind.
	_, err := db.Exec(`ALTER TABLE token_accounts ADD COLUMN balance_milli BIGINT`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE token_accounts SET balance_milli = 1 WHERE id = 1`)
	require.NoError(t, err)

	rep, err := migration.Upgrade(context.Background(), db, migration.DialectSQLite)
	require.Error(t, err)
	require.NotEqual(t, 0, rep.Mismatches)
	assert.ErrorContains(t, err, "mismatch")

	// Float column retained (NOT dropped) and milli column still present.
	var bal float64
	require.NoError(t, db.QueryRow(`SELECT balance FROM token_accounts WHERE id = 1`).Scan(&bal))
	assert.Equal(t, 25.50, bal)

	var milliCol string
	require.NoError(t, db.QueryRow(`SELECT name FROM pragma_table_info('token_accounts') WHERE name = 'balance_milli'`).Scan(&milliCol))
	assert.Equal(t, "balance_milli", milliCol)
}

// TestMigrateMilli_UpgradeIdempotent — re-running Upgrade after a successful
// run is a no-op: no double-backfill (25500 must NOT become 25500000), no
// error, no corruption (spec: migration SHALL be reversible and safe to re-run).
func TestMigrateMilli_UpgradeIdempotent(t *testing.T) {
	db := newMigrationDB(t)
	defer db.Close()
	seedLegacyRows(t, db)

	_, err := migration.Upgrade(context.Background(), db, migration.DialectSQLite)
	require.NoError(t, err)

	rep2, err := migration.Upgrade(context.Background(), db, migration.DialectSQLite)
	require.NoError(t, err)
	require.Equal(t, 0, rep2.Mismatches)

	var bal int64
	require.NoError(t, db.QueryRow(`SELECT balance FROM token_accounts WHERE id = 1`).Scan(&bal))
	assert.Equal(t, int64(25500), bal) // not doubled, not corrupted
}

// TestMigrateMilli_DownRestoresFloats — Downgrade reverses Upgrade: the
// bigint milli column becomes a float column with legacy decimal values
// restored via /1000.0 (design: down-migration `* = *_milli / 1000.0`).
func TestMigrateMilli_DownRestoresFloats(t *testing.T) {
	db := newMigrationDB(t)
	defer db.Close()
	seedLegacyRows(t, db)

	_, err := migration.Upgrade(context.Background(), db, migration.DialectSQLite)
	require.NoError(t, err)

	_, err = migration.Downgrade(context.Background(), db, migration.DialectSQLite)
	require.NoError(t, err)

	var bal float64
	require.NoError(t, db.QueryRow(`SELECT balance FROM token_accounts WHERE id = 1`).Scan(&bal))
	assert.Equal(t, 25.50, bal)

	var bal2 float64
	require.NoError(t, db.QueryRow(`SELECT balance FROM token_accounts WHERE id = 2`).Scan(&bal2))
	assert.InDelta(t, 25.556, bal2, 1e-9) // 25556 / 1000.0

	// SQLite has no ALTER COLUMN TYPE: values are re-stored as REAL (dynamic
	// typing), which is the storage class the legacy schema read anyway.
	var storageType string
	require.NoError(t, db.QueryRow(`SELECT typeof(balance) FROM token_accounts WHERE id = 1`).Scan(&storageType))
	assert.Equal(t, "real", storageType)
}

// TestMigrateMilli_CoversAllElevenColumns — the migration converts every one
// of the 11 design columns across the 5 tables (design Phase 4 list).
func TestMigrateMilli_CoversAllElevenColumns(t *testing.T) {
	got := make(map[string]map[string]bool)
	for _, c := range migration.MoneyColumns {
		if got[c.Table] == nil {
			got[c.Table] = map[string]bool{}
		}
		got[c.Table][c.Name] = true
	}
	expected := map[string][]string{
		"token_accounts":     {"balance", "total_purchased", "total_consumed"},
		"token_transactions": {"amount", "balance_before", "balance_after"},
		"token_top_ups":      {"tokens_received", "bonus_tokens"},
		"token_missions":     {"reward_tokens"},
		"referrals":          {"reward_referrer", "reward_referred"},
	}
	total := 0
	for table, cols := range expected {
		for _, col := range cols {
			assert.True(t, got[table][col], fmt.Sprintf("missing %s.%s", table, col))
			total++
		}
	}
	assert.Equal(t, 11, total)
}
