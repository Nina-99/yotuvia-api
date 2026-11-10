// Command migrate_milli converts the token-ledger money columns from float to
// int64 milli-Ytv on the production PostgreSQL database.
//
// Usage:
//
//	go run ./scripts/migrate_milli.go            # additive, verified, reversible up-migration
//	go run ./scripts/migrate_milli.go --down     # restore floats (*_milli / 1000.0)
//
// The database connection is read from the same env vars the application uses
// (DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME; .env is loaded via
// godotenv, mirroring configs/db.config.go). Override with DB_DSN if set.
//
// The up-migration is idempotent: re-running after a partial/failed run
// resumes from the checkpoint and never double-converts or corrupts data.
// Verification aborts the run (non-zero exit) on any mismatch BEFORE float
// columns are dropped — data is never silently discarded.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Nina-99/yotuvia-api/migration"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

func dsnFromEnv() string {
	if dsn := os.Getenv("DB_DSN"); dsn != "" {
		return dsn
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"))
}

func main() {
	down := flag.Bool("down", false, "restore legacy float columns (*_milli / 1000.0)")
	flag.Parse()

	// .env is optional: env vars may already be exported (CI, containers).
	_ = godotenv.Load()

	db, err := sql.Open("pgx", dsnFromEnv())
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate_milli: open: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "migrate_milli: cannot reach database: %v\n", err)
		os.Exit(1)
	}

	var rep *migration.Report
	if *down {
		rep, err = migration.Downgrade(ctx, db, migration.DialectPostgres)
	} else {
		rep, err = migration.Upgrade(ctx, db, migration.DialectPostgres)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate_milli: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("migrate_milli: %s complete — columns added: %d, rows backfilled: %d, "+
		"mismatches: %d, tables cut over: %d, already in target state: %d\n",
		rep.Direction, rep.ColumnsAdded, rep.RowsBackfilled, rep.Mismatches, rep.TablesCutOver, rep.Skipped)
}
