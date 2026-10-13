package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Golden SQL fragments for the shared rating scope. The scope aggregates
// reviews ONCE with a grouped LEFT JOIN (the same shape the ranking query uses)
// instead of a correlated subquery per centre per column, because reviews has no
// index on beauty_center_id and the list endpoints paginate up to 100 rows.
//
// The single-pass aggregate is the contract: the join must appear verbatim, one
// `reviews` reference per statement, and avg_rating must precede review_count.
const (
	goldenRatingJoin     = "LEFT JOIN (SELECT beauty_center_id, AVG(rating) AS avg_rating, COUNT(*) AS review_count FROM reviews WHERE deleted_at IS NULL GROUP BY beauty_center_id) rating_agg ON rating_agg.beauty_center_id = beauty_centers.id"
	goldenAvgSelect      = "COALESCE(rating_agg.avg_rating, 0) AS avg_rating"
	goldenCountSelect    = "COALESCE(rating_agg.review_count, 0) AS review_count"
	goldenDistanceSelect = "(6371 * acos(cos(radians(?)) * cos(radians(latitude)) * cos(radians(longitude) - radians(?)) + sin(radians(?)) * sin(radians(latitude)))) AS distance"
)

// sqlRecorder captures every SQL statement a GORM session emits via the logger
// Trace hook. DryRun sessions skip execution but still build SQL and trace it.
type sqlRecorder struct {
	logger.Interface
	queries []string
}

func (r *sqlRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	r.queries = append(r.queries, sql)
}

// TestBeautyCenterRatingMethodsEmitGoldenSQL is an approval test for the shared
// rating scope: each public rating query must emit the single grouped
// aggregate. Search is covered at the scope level instead — under GORM DryRun a
// preceding Count leaves its SQL on the shared statement, so the find query is
// not traced for Search (test-harness artifact only; production resets the
// statement after every executed query).
func TestBeautyCenterRatingMethodsEmitGoldenSQL(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	rec := &sqlRecorder{Interface: logger.Default}
	repo := NewBeautyCenterRepositoryImpl(db.Session(&gorm.Session{DryRun: true, Logger: rec}))

	t.Run("FindByID", func(t *testing.T) {
		_, err := repo.FindByID(1)
		assert.NoError(t, err)
		assertGoldenRatingSQL(t, rec)
	})

	t.Run("FindAll", func(t *testing.T) {
		_, _, err := repo.FindAll(1, 10)
		assert.NoError(t, err)
		assertGoldenRatingSQL(t, rec)
	})

	t.Run("FindTopRated", func(t *testing.T) {
		_, err := repo.FindTopRated(5)
		assert.NoError(t, err)
		assertGoldenRatingSQL(t, rec)
	})

	t.Run("FindNearby", func(t *testing.T) {
		_, err := repo.FindNearby(10.5, -66.0, 5)
		assert.NoError(t, err)
		assertGoldenRatingSQL(t, rec)
		assert.Contains(t, strings.Join(rec.queries, "\n"), "AS distance")
	})
}

func assertGoldenRatingSQL(t *testing.T, rec *sqlRecorder) {
	t.Helper()
	ratingQueries := 0
	for _, q := range rec.queries {
		if !strings.Contains(q, goldenRatingJoin) {
			continue
		}
		ratingQueries++
		assert.Contains(t, q, goldenAvgSelect)
		assert.Contains(t, q, goldenCountSelect)
		assert.Equal(t, 1, strings.Count(q, "reviews"),
			"reviews must be aggregated exactly once, not per centre")
	}
	assert.Positive(t, ratingQueries, "expected a query carrying the rating aggregate")
}

// TestWithAverageRatingScopeGoldenSQL pins the withAverageRating scope to the
// exact SQL of the single grouped aggregate it owns. Covers the plain shape
// (FindByID/FindAll/Search/FindTopRated) and the extra-SELECT shape with bind
// args (FindNearby).
func TestWithAverageRatingScopeGoldenSQL(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}

	build := func(t *testing.T, apply func(tx *gorm.DB) *gorm.DB) (string, []interface{}) {
		t.Helper()
		tx := db.Session(&gorm.Session{DryRun: true})
		tx = apply(tx)
		if err := tx.Find(&models.BeautyCenter{}).Error; err != nil {
			t.Fatalf("build query: %v", err)
		}
		return tx.Statement.SQL.String(), tx.Statement.Vars
	}

	t.Run("plain select (FindByID/FindAll/Search/FindTopRated shape)", func(t *testing.T) {
		sql, vars := build(t, func(tx *gorm.DB) *gorm.DB {
			return tx.Scopes(withAverageRating(""))
		})
		assert.Contains(t, sql, goldenRatingJoin)
		assert.Contains(t, sql, goldenAvgSelect)
		assert.Contains(t, sql, goldenCountSelect)
		// Column order preserved: avg_rating must precede review_count.
		assert.Less(t, strings.Index(sql, goldenAvgSelect), strings.Index(sql, goldenCountSelect))
		assert.Equal(t, 1, strings.Count(sql, "reviews"))
		assert.Empty(t, vars)
	})

	t.Run("extra select appended with bind args (FindNearby shape)", func(t *testing.T) {
		sql, vars := build(t, func(tx *gorm.DB) *gorm.DB {
			return tx.Scopes(withAverageRating(goldenDistanceSelect, 10.5, -66.0, 10.5))
		})
		assert.Contains(t, sql, goldenRatingJoin)
		assert.Contains(t, sql, goldenAvgSelect)
		assert.Contains(t, sql, goldenCountSelect)
		assert.Contains(t, sql, goldenDistanceSelect)
		assert.Equal(t, []interface{}{10.5, -66.0, 10.5}, vars)
	})
}
