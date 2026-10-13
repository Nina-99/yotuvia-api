// Package migration implements the additive, verified, reversible conversion
// of the token-ledger money columns from float to int64 milli-YOTTUs.
//
// The engine is dialect-agnostic: production runs against PostgreSQL via
// scripts/migrate_milli.go (DialectPostgres), while the integration tests run
// the exact same code path against in-memory SQLite (DialectSQLite). Only the
// column-type introspection and type-casting statements differ per dialect;
// the backfill/verify math is pure Go (utils.ToMilli) and therefore identical
// on both engines and to the application writes.
package migration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Nina-99/yotuvia-api/utils"
)

// Dialect identifies the SQL engine the migration runs against.
type Dialect string

const (
	// DialectPostgres is the production dialect (scripts/migrate_milli.go).
	DialectPostgres Dialect = "postgres"
	// DialectSQLite is the CI-test dialect (tests/migrate_milli_test.go).
	DialectSQLite Dialect = "sqlite"
)

// MoneyColumn identifies one persisted legacy float money field.
type MoneyColumn struct {
	Table string
	Name  string
}

// MoneyColumns lists the 11 float money columns converted to milli-YOTTUs
// (design Phase 4). Tables/columns come from this fixed list only; they are
// never derived from user input, so interpolating them into SQL is safe.
var MoneyColumns = []MoneyColumn{
	{Table: "token_accounts", Name: "balance"},
	{Table: "token_accounts", Name: "total_purchased"},
	{Table: "token_accounts", Name: "total_consumed"},
	{Table: "token_transactions", Name: "amount"},
	{Table: "token_transactions", Name: "balance_before"},
	{Table: "token_transactions", Name: "balance_after"},
	{Table: "token_top_ups", Name: "tokens_received"}, // GORM pluralization: TopUp -> token_top_ups
	{Table: "token_top_ups", Name: "bonus_tokens"},
	{Table: "token_missions", Name: "reward_tokens"},
	{Table: "referrals", Name: "reward_referrer"},
	{Table: "referrals", Name: "reward_referred"},
}

// BackfillMilli converts a legacy decimal YOTTU value to milli-YOTTUs using
// the same canonical rounding the application uses (utils.ToMilli): 6-decimal
// quantization then half-away-from-zero rounding. Delegating guarantees the
// migration backfill always matches what the app writes (spec: 25.50 -> 25500,
// 25.5555 -> 25556), instead of drifting via engine-specific SQL ROUND.
func BackfillMilli(v float64) int64 {
	return utils.ToMilli(v)
}

// Report summarizes one Upgrade or Downgrade run.
type Report struct {
	Direction      string // "up" or "down"
	ColumnsAdded   int    // *_milli columns created
	RowsBackfilled int64  // rows backfilled
	Mismatches     int    // rows failing verification (up only)
	TablesCutOver  int    // tables whose floats were dropped and columns renamed
	Skipped        int    // columns already in target state (idempotency)
}

// sqlDB is the subset of *sql.DB / *sql.Tx used by the engine.
type sqlDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// placeholder returns the bind-parameter token for the Nth argument: pgx
// (Postgres) uses $1-style, mattn/go-sqlite3 uses ?.
func placeholder(d Dialect, n int) string {
	if d == DialectPostgres {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

func isIntegerType(t string) bool {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "bigint", "integer", "int", "int8", "int64", "int4":
		return true
	}
	return false
}

func isFloatType(t string) bool {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "numeric", "decimal", "double precision", "real", "float", "float8", "float4", "double":
		return true
	}
	return false
}

// columnInfo returns the declared type of table.col and whether it exists.
func columnInfo(ctx context.Context, db sqlDB, d Dialect, table, col string) (string, bool, error) {
	var dataType string
	var err error
	switch d {
	case DialectPostgres:
		err = db.QueryRowContext(ctx,
			`SELECT data_type FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`,
			table, col).Scan(&dataType)
	case DialectSQLite:
		err = db.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT type FROM pragma_table_info('%s') WHERE name = ?`, table), col).Scan(&dataType)
	default:
		return "", false, fmt.Errorf("migration: unknown dialect %q", d)
	}
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("migration: introspect %s.%s: %w", table, col, err)
	}
	return dataType, true, nil
}

func exec(ctx context.Context, db sqlDB, query string, args ...any) error {
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("migration: %s: %w", query, err)
	}
	return nil
}

// Upgrade converts the 11 money columns to milli-YOTTUs:
//
//  1. Additive: create *_milli bigint columns where missing (idempotent);
//     recover a crash between DROP and RENAME from a previous run.
//  2. Backfill: for each row where *_milli IS NULL compute the canonical milli
//     value from the legacy float and write it.
//  3. Verify: recompute every backfilled value; ANY mismatch aborts the run
//     BEFORE any float column is dropped — data is never silently discarded.
//  4. Cutover: drop the float column and rename *_milli to the original name.
//
// Re-running after a partial/failed run resumes from the checkpoint: existing
// *_milli columns are backfilled only where NULL, and already-converted
// columns are skipped, so no data is double-converted or corrupted.
func Upgrade(ctx context.Context, db *sql.DB, d Dialect) (*Report, error) {
	rep := &Report{Direction: "up"}

	// Pass 1 — additive + backfill (idempotent).
	for _, mc := range MoneyColumns {
		origType, origExists, err := columnInfo(ctx, db, d, mc.Table, mc.Name)
		if err != nil {
			return rep, err
		}
		_, milliExists, err := columnInfo(ctx, db, d, mc.Table, mc.Name+"_milli")
		if err != nil {
			return rep, err
		}

		switch {
		case !origExists && !milliExists:
			return rep, fmt.Errorf("migration: %s.%s: column not found in schema", mc.Table, mc.Name)
		case !origExists && milliExists:
			// Crash recovery: a prior run dropped the float but died before the rename.
			if err := exec(ctx, db, fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
				mc.Table, mc.Name+"_milli", mc.Name)); err != nil {
				return rep, err
			}
			rep.TablesCutOver++
			continue
		case !milliExists && isIntegerType(origType):
			rep.Skipped++ // already migrated
			continue
		case !milliExists:
			if err := exec(ctx, db, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s BIGINT",
				mc.Table, mc.Name+"_milli")); err != nil {
				return rep, err
			}
			rep.ColumnsAdded++
		case isIntegerType(origType):
			// Original already bigint but a stale *_milli column exists: drop it.
			if err := exec(ctx, db, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s",
				mc.Table, mc.Name+"_milli")); err != nil {
				return rep, err
			}
			rep.Skipped++
			continue
		}

		n, err := backfill(ctx, db, d, mc)
		if err != nil {
			return rep, err
		}
		rep.RowsBackfilled += n
	}

	// Pass 2 — verify. Abort BEFORE any drop on mismatch.
	if err := verify(ctx, db, d, rep); err != nil {
		return rep, err
	}

	// Pass 3 — cutover: drop floats, rename *_milli to original names.
	tables := make(map[string]bool)
	for _, mc := range MoneyColumns {
		tables[mc.Table] = true
	}
	for table := range tables {
		if err := cutoverTable(ctx, db, d, table, rep); err != nil {
			return rep, err
		}
	}
	return rep, nil
}

// backfill computes the milli value for rows where *_milli IS NULL, using the
// canonical BackfillMilli math (NOT engine-specific SQL ROUND). Rows with a
// NULL legacy float are left NULL (nothing to convert, nothing to lose).
// The scan is drained before any UPDATE so a single-connection DB pool (or a
// SQLite :memory: DB pinned to one connection) does not deadlock.
func backfill(ctx context.Context, db sqlDB, d Dialect, mc MoneyColumn) (int64, error) {
	rows, err := db.QueryContext(ctx,
		fmt.Sprintf("SELECT id, %s FROM %s WHERE %s_milli IS NULL AND %s IS NOT NULL",
			mc.Name, mc.Table, mc.Name, mc.Name))
	if err != nil {
		return 0, fmt.Errorf("migration: backfill scan %s.%s: %w", mc.Table, mc.Name, err)
	}

	var targets []struct {
		id    int64
		value float64
	}
	for rows.Next() {
		var t struct {
			id    int64
			value float64
		}
		if err := rows.Scan(&t.id, &t.value); err != nil {
			rows.Close()
			return 0, fmt.Errorf("migration: backfill scan %s.%s: %w", mc.Table, mc.Name, err)
		}
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("migration: backfill scan %s.%s: %w", mc.Table, mc.Name, err)
	}
	rows.Close()

	for _, t := range targets {
		milli := BackfillMilli(t.value)
		if err := exec(ctx, db,
			fmt.Sprintf("UPDATE %s SET %s_milli = %s WHERE id = %s",
				mc.Table, mc.Name, placeholder(d, 1), placeholder(d, 2)), milli, t.id); err != nil {
			return 0, err
		}
	}
	return int64(len(targets)), nil
}

// verify recomputes every backfilled value and counts mismatches. On any
// mismatch the run aborts BEFORE the cutover, retaining the float columns.
func verify(ctx context.Context, db sqlDB, d Dialect, rep *Report) error {
	var firstErr string
	for _, mc := range MoneyColumns {
		origType, origExists, err := columnInfo(ctx, db, d, mc.Table, mc.Name)
		if err != nil {
			return err
		}
		_, milliExists, err := columnInfo(ctx, db, d, mc.Table, mc.Name+"_milli")
		if err != nil {
			return err
		}
		if !origExists || !milliExists || isIntegerType(origType) {
			continue // nothing to verify (already migrated or not additive yet)
		}
		rows, err := db.QueryContext(ctx,
			fmt.Sprintf("SELECT id, %s, %s_milli FROM %s WHERE %s IS NOT NULL",
				mc.Name, mc.Name, mc.Table, mc.Name))
		if err != nil {
			return fmt.Errorf("migration: verify scan %s.%s: %w", mc.Table, mc.Name, err)
		}
		for rows.Next() {
			var id int64
			var v float64
			var milli sql.NullInt64
			if err := rows.Scan(&id, &v, &milli); err != nil {
				rows.Close()
				return fmt.Errorf("migration: verify scan %s.%s: %w", mc.Table, mc.Name, err)
			}
			expected := BackfillMilli(v)
			if !milli.Valid || milli.Int64 != expected {
				rep.Mismatches++
				if firstErr == "" {
					firstErr = fmt.Sprintf("%s.%s id=%d: milli=%v expected=%d",
						mc.Table, mc.Name, id, milli, expected)
				}
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	}
	if rep.Mismatches > 0 {
		return fmt.Errorf("migration: verification mismatch (%d row(s), first: %s); "+
			"float columns retained, cutover aborted", rep.Mismatches, firstErr)
	}
	return nil
}

// cutoverTable drops the float columns and renames the *_milli columns to the
// original names for one table, atomically per table. All introspection
// happens BEFORE the transaction opens, so the tx can hold the single
// connection of a SQLite :memory: DB without deadlocking.
func cutoverTable(ctx context.Context, db *sql.DB, d Dialect, table string, rep *Report) error {
	var cols []MoneyColumn
	for _, mc := range MoneyColumns {
		if mc.Table != table {
			continue
		}
		origType, origExists, err := columnInfo(ctx, db, d, table, mc.Name)
		if err != nil {
			return err
		}
		_, milliExists, err := columnInfo(ctx, db, d, table, mc.Name+"_milli")
		if err != nil {
			return err
		}
		if origExists && milliExists && isFloatType(origType) {
			cols = append(cols, mc)
		}
	}
	if len(cols) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migration: begin cutover %s: %w", table, err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	for _, mc := range cols {
		if err := exec(ctx, tx, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, mc.Name)); err != nil {
			return err
		}
		if err := exec(ctx, tx, fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
			table, mc.Name+"_milli", mc.Name)); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migration: commit cutover %s: %w", table, err)
	}
	rep.TablesCutOver++
	return nil
}

// Downgrade reverses Upgrade: restored float columns hold *_milli / 1000.0
// (sub-milli precision loss is documented in the design). Also drops leftover
// *_milli columns from a partial upgrade, leaving the original floats intact.
func Downgrade(ctx context.Context, db *sql.DB, d Dialect) (*Report, error) {
	rep := &Report{Direction: "down"}

	for _, mc := range MoneyColumns {
		origType, origExists, err := columnInfo(ctx, db, d, mc.Table, mc.Name)
		if err != nil {
			return rep, err
		}
		_, milliExists, err := columnInfo(ctx, db, d, mc.Table, mc.Name+"_milli")
		if err != nil {
			return rep, err
		}

		switch {
		case !origExists && !milliExists:
			return rep, fmt.Errorf("migration: %s.%s: column not found in schema", mc.Table, mc.Name)
		case milliExists && isFloatType(origType):
			// Partial upgrade: original floats untouched, stale milli column present.
			if err := exec(ctx, db, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s",
				mc.Table, mc.Name+"_milli")); err != nil {
				return rep, err
			}
			rep.Skipped++
			continue
		case !origExists && milliExists:
			// Crash between DROP and RENAME: restore the name, then convert below.
			if err := exec(ctx, db, fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
				mc.Table, mc.Name+"_milli", mc.Name)); err != nil {
				return rep, err
			}
			origType, _, err = columnInfo(ctx, db, d, mc.Table, mc.Name)
			if err != nil {
				return rep, err
			}
			if milliExists = isIntegerType(origType); !milliExists {
				rep.Skipped++
				continue
			}
		case isFloatType(origType):
			rep.Skipped++ // already floats
			continue
		}

		// Fully migrated: convert bigint milli back to float / 1000.0.
		if err := restoreFloat(ctx, db, d, mc); err != nil {
			return rep, err
		}
		rep.TablesCutOver++
	}
	return rep, nil
}

// restoreFloat converts the migrated bigint column back to a float column
// holding *_milli / 1000.0 (design rollback formula).
func restoreFloat(ctx context.Context, db sqlDB, d Dialect, mc MoneyColumn) error {
	switch d {
	case DialectPostgres:
		return exec(ctx, db,
			fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE double precision USING %s / 1000.0",
				mc.Table, mc.Name, mc.Name))
	case DialectSQLite:
		// SQLite has no ALTER COLUMN TYPE; values are re-stored as REAL (dynamic
		// typing), which is what the legacy schema read anyway.
		return exec(ctx, db,
			fmt.Sprintf("UPDATE %s SET %s = %s / 1000.0 WHERE %s IS NOT NULL",
				mc.Table, mc.Name, mc.Name, mc.Name))
	default:
		return fmt.Errorf("migration: unknown dialect %q", d)
	}
}
