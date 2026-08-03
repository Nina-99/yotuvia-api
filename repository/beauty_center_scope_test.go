package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Golden SQL fragments, captured verbatim from the pre-refactor inline SELECT
// expressions in beauty_center.repository.impl.go (avg_rating/review_count
// subqueries at lines 38/58/156/171/187, distance in FindNearby). The rating
// scope refactor (task 3.3) MUST reproduce these byte-for-byte; any drift in the
// generated SQL fails the approval assertions below.
const (
	goldenAvgSelect      = "COALESCE((SELECT AVG(rating)::numeric(10,2) FROM reviews WHERE beauty_center_id = beauty_centers.id), 0) as avg_rating"
	goldenCountSelect    = "COALESCE((SELECT COUNT(*) FROM reviews WHERE beauty_center_id = beauty_centers.id), 0) as review_count"
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

// TestBeautyCenterRatingMethodsEmitGoldenSQL is an approval test for the rating
// scope refactor (task 3.3): each public rating query must keep emitting the
// pre-refactor subquery SQL. Search is covered at the scope level instead —
// under GORM DryRun a preceding Count leaves its SQL on the shared statement,
// so the find query is not traced for Search (test-harness artifact only;
// production resets the statement after every executed query).
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
	all := strings.Join(rec.queries, "\n")
	assert.Contains(t, all, goldenAvgSelect)
	assert.Contains(t, all, goldenCountSelect)
}

// TestWithAverageRatingScopeGoldenSQL pins the withAverageRating scope to the
// exact SQL of the inline subqueries it replaces (approval test). The scope did
// not exist when this test was written, so it failed to compile (RED); it is the
// new unit introduced by the refactor. Covers the plain shape (FindByID/FindAll/
// Search/FindTopRated) and the extra-SELECT shape with bind args (FindNearby).
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
		assert.Contains(t, sql, goldenAvgSelect)
		assert.Contains(t, sql, goldenCountSelect)
		// Column order preserved: avg_rating must precede review_count.
		assert.Less(t, strings.Index(sql, goldenAvgSelect), strings.Index(sql, goldenCountSelect))
		assert.Empty(t, vars)
	})

	t.Run("extra select appended with bind args (FindNearby shape)", func(t *testing.T) {
		sql, vars := build(t, func(tx *gorm.DB) *gorm.DB {
			return tx.Scopes(withAverageRating(goldenDistanceSelect, 10.5, -66.0, 10.5))
		})
		assert.Contains(t, sql, goldenAvgSelect)
		assert.Contains(t, sql, goldenCountSelect)
		assert.Contains(t, sql, goldenDistanceSelect)
		assert.Equal(t, []interface{}{10.5, -66.0, 10.5}, vars)
	})
}
