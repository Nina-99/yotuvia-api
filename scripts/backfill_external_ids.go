//go:build ignore

// Command backfill_external_ids fills feed_posts.external_id for external
// (tiktok|youtube|instagram) posts that were linked BEFORE the API learned to
// resolve provider short links.
//
// A share/short link (TikTok's vt.tiktok.com / vm.tiktok.com / tiktok.com/t/
// forms) carries no post id in its path, so it was stored with an empty
// external_id. Without an id the clients cannot build an official embed and
// silently fall back to a deep link — which is what makes a short-link post
// "open the TikTok app" instead of playing.
//
// For each affected row this command follows the redirect to the canonical URL
// and stores both the canonical URL and the post id extracted from it. A row
// whose id still cannot be derived (link dead, provider changed) is reported
// and left untouched.
//
// Usage:
//
//	go run ./scripts/backfill_external_ids.go            # dry run, report only
//	go run ./scripts/backfill_external_ids.go --apply    # persist the changes
//
// Safe to re-run: only rows with an empty external_id are selected, so rows
// already fixed are never revisited.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Nina-99/yotuvia-api/service"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

// resolveTimeout bounds one provider redirect chase so a single dead link
// cannot stall the whole run.
const resolveTimeout = 15 * time.Second

func dsnFromEnv() string {
	if dsn := os.Getenv("DB_DSN"); dsn != "" {
		return dsn
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"))
}

type row struct {
	id          int64
	provider    string
	externalURL string
}

func main() {
	apply := flag.Bool("apply", false, "persist the resolved ids (default: dry run)")
	flag.Parse()

	// .env is optional: env vars may already be exported (CI, containers).
	_ = godotenv.Load()

	db, err := sql.Open("pgx", dsnFromEnv())
	if err != nil {
		fmt.Fprintf(os.Stderr, "backfill_external_ids: open: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "backfill_external_ids: cannot reach database: %v\n", err)
		os.Exit(1)
	}

	rows, err := selectPending(ctx, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backfill_external_ids: select: %v\n", err)
		os.Exit(1)
	}

	if len(rows) == 0 {
		fmt.Println("backfill_external_ids: nothing to do — every external post already has an id")
		return
	}

	resolver := service.NewHTTPOEmbedResolver(nil)
	var resolved, unresolved, failed int

	for _, r := range rows {
		canonical, id, err := resolve(ctx, resolver, r)
		if err != nil {
			failed++
			fmt.Printf("  FAIL id=%d provider=%s %s — %v\n", r.id, r.provider, r.externalURL, err)
			continue
		}
		if id == "" {
			unresolved++
			fmt.Printf("  SKIP id=%d provider=%s %s — id not derivable\n", r.id, r.provider, r.externalURL)
			continue
		}

		resolved++
		fmt.Printf("  OK   id=%d provider=%s\n       %s -> %s\n", r.id, r.provider, r.externalURL, canonical)

		if *apply {
			if _, err := db.ExecContext(ctx,
				`UPDATE feed_posts SET external_id = $1, external_url = $2, updated_at = now() WHERE id = $3`,
				id, canonical, r.id,
			); err != nil {
				fmt.Fprintf(os.Stderr, "backfill_external_ids: update id=%d: %v\n", r.id, err)
				os.Exit(1)
			}
		}
	}

	mode := "DRY RUN (nothing written — re-run with --apply)"
	if *apply {
		mode = "APPLIED"
	}
	fmt.Printf("backfill_external_ids: %s — candidates: %d, resolved: %d, unresolved: %d, failed: %d\n",
		mode, len(rows), resolved, unresolved, failed)

	if !*apply && resolved > 0 {
		fmt.Println("backfill_external_ids: re-run with --apply to persist")
	}
}

func selectPending(ctx context.Context, db *sql.DB) ([]row, error) {
	const q = `
		SELECT id, provider, external_url
		FROM feed_posts
		WHERE deleted_at IS NULL
		  AND provider IN ('tiktok', 'youtube', 'instagram')
		  AND (external_id IS NULL OR external_id = '')
		  AND external_url IS NOT NULL AND external_url <> ''
		ORDER BY id`
	rs, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	var out []row
	for rs.Next() {
		var r row
		if err := rs.Scan(&r.id, &r.provider, &r.externalURL); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

// resolve follows the provider's short link (when needed) and returns the
// canonical URL plus the id derivable from it.
func resolve(ctx context.Context, resolver service.OEmbedResolver, r row) (string, string, error) {
	// An already-canonical link needs no request: the id is right there.
	if id := service.ExternalIDFromURL(r.externalURL); id != "" {
		return r.externalURL, id, nil
	}

	rctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()

	canonical := resolver.Canonicalize(rctx, r.externalURL)
	// Only trust a redirect that stays on the same supported provider.
	if canonical == "" || service.ProviderFromURL(canonical) != r.provider {
		canonical = r.externalURL
	}
	return canonical, service.ExternalIDFromURL(canonical), nil
}
