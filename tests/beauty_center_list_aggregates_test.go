package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ─────────────────────────────────────────────────────────────────────────────
//  List-family aggregate contract (avg_rating / review_count / schedules)
//
//  Regression guard for the bug where GET /beauty-centers, /search, /top-rated
//  and /nearby returned avg_rating=0, review_count=0 and schedules=null while
//  the data existed and the ranking endpoint reported the real values.
// ─────────────────────────────────────────────────────────────────────────────

// queryCounter counts every SQL statement GORM emits for a request, so a
// per-centre N+1 regression surfaces as a count that grows with the page size.
type queryCounter struct {
	logger.Interface
	sqls []string
}

func (q *queryCounter) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	q.sqls = append(q.sqls, sql)
}

func (q *queryCounter) reset()      { q.sqls = nil }
func (q *queryCounter) count() int  { return len(q.sqls) }
func (q *queryCounter) all() string { return strings.Join(q.sqls, "\n") }

// registerSQLiteMath makes the SQLite test driver able to evaluate the math
// functions the nearby (acos/cos/sin/radians) and ranking (log) queries use.
// Production runs PostgreSQL, which has them natively; SQLite only needs them
// to execute those two endpoints in-harness.
func registerSQLiteMath(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	conn, err := sqlDB.Conn(context.Background())
	require.NoError(t, err)
	defer conn.Close()

	toF := func(v any) float64 {
		switch n := v.(type) {
		case int64:
			return float64(n)
		case float64:
			return n
		}
		return 0
	}
	funcs := map[string]any{
		"acos":    func(x any) float64 { return math.Acos(toF(x)) },
		"cos":     func(x any) float64 { return math.Cos(toF(x)) },
		"sin":     func(x any) float64 { return math.Sin(toF(x)) },
		"radians": func(x any) float64 { return toF(x) * math.Pi / 180 },
		"log":     func(x any) float64 { return math.Log10(toF(x)) },
		"ln":      func(x any) float64 { return math.Log(toF(x)) },
	}
	require.NoError(t, conn.Raw(func(driverConn any) error {
		c, ok := driverConn.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("unexpected driver conn %T", driverConn)
		}
		for name, f := range funcs {
			if e := c.RegisterFunc(name, f, true); e != nil {
				return fmt.Errorf("register %s: %w", name, e)
			}
		}
		return nil
	}))
}

// beautyCenterListTestRouter mounts BeautyCenterRoutes over the shared SQLite
// schema with a query counter attached to the GORM logger.
func beautyCenterListTestRouter(t *testing.T) (*gin.Engine, *gorm.DB, *queryCounter) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	registerSQLiteMath(t, db)
	rec := &queryCounter{Interface: logger.Default}
	db.Logger = rec
	r := gin.New()
	r.Use(gin.Recovery())
	router.BeautyCenterRoutes(r.Group("/api"), db, validator.New())
	return r, db, rec
}

// seedListCenter creates an active centre with the given ratings and working
// blocks. Ratings are the values the client left; working blocks are
// (day, start, end) triples.
func seedListCenter(t *testing.T, db *gorm.DB, id uint, name string, lat, lng float64, ratings []int, blocks [][3]any) {
	t.Helper()
	center := &models.BeautyCenter{
		ID:           id,
		Name:         name,
		Email:        fmt.Sprintf("list-center-%d@test.com", id),
		Phone:        fmt.Sprintf("555%04d", id),
		IsActive:     true,
		Latitude:     lat,
		Longitude:    lng,
		ReferralCode: fmt.Sprintf("LREF%03d", id),
	}
	require.NoError(t, db.Create(center).Error)

	for i, rating := range ratings {
		client := &models.Client{
			Model:    gorm.Model{ID: 9000 + id*10 + uint(i)},
			Username: fmt.Sprintf("listc-%d-%d", id, i),
			Email:    fmt.Sprintf("list-client-%d-%d@test.com", id, i),
			Phone:    fmt.Sprintf("5559%04d", id*10+uint(i)),
			IsActive: true,
		}
		require.NoError(t, db.Create(client).Error)
		require.NoError(t, db.Create(&models.Review{
			Model:          gorm.Model{ID: 7000 + id*10 + uint(i)},
			BeautyCenterID: id,
			ClientID:       client.ID,
			Rating:         rating,
		}).Error)
	}

	for i, b := range blocks {
		require.NoError(t, db.Create(&models.WorkingHour{
			Model:          gorm.Model{ID: 6000 + id*10 + uint(i)},
			BeautyCenterID: id,
			DayOfWeek:      b[0].(int),
			StartTime:      b[1].(string),
			EndTime:        b[2].(string),
		}).Error)
	}
}

// listEndpoint names one list-family centre endpoint and whether its body is
// wrapped in the pagination envelope.
type listEndpoint struct {
	name      string
	path      string
	paginated bool
}

func listCenterEndpoints() []listEndpoint {
	return []listEndpoint{
		{"list", "/api/beauty-centers?page=1&limit=10", true},
		{"search", "/api/beauty-centers/search?q=Center&page=1&limit=10", true},
		{"top-rated", "/api/beauty-centers/top-rated?limit=10", false},
		{"nearby", "/api/beauty-centers/nearby?lat=10&lng=20&limit=10", false},
	}
}

func fetchCenters(t *testing.T, r *gin.Engine, ep listEndpoint) []response.BeautyCenterResponse {
	t.Helper()
	w := do(r, http.MethodGet, ep.path, "", "")
	require.Equal(t, http.StatusOK, w.Code, "%s body=%s", ep.name, w.Body.String())
	if ep.paginated {
		var env struct {
			Data []response.BeautyCenterResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), "%s body=%s", ep.name, w.Body.String())
		return env.Data
	}
	var arr []response.BeautyCenterResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &arr), "%s body=%s", ep.name, w.Body.String())
	return arr
}

func centerByID(t *testing.T, centers []response.BeautyCenterResponse, id uint) response.BeautyCenterResponse {
	t.Helper()
	for _, c := range centers {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("centre %d not present in payload (got %d centres)", id, len(centers))
	return response.BeautyCenterResponse{}
}

// TestBeautyCenterListEndpoints_IncludeRatingAggregates — a centre with reviews
// must report its real avg_rating/review_count from EVERY list-family endpoint;
// a centre without reviews must report 0 without erroring.
func TestBeautyCenterListEndpoints_IncludeRatingAggregates(t *testing.T) {
	r, db, _ := beautyCenterListTestRouter(t)
	seedListCenter(t, db, 1, "Rated Center", 10, 20, []int{5, 4, 3}, nil) // avg 4.0, 3 reviews
	seedListCenter(t, db, 2, "Unrated Center", 11, 21, nil, nil)          // 0 / 0
	seedListCenter(t, db, 3, "Solo Center", 12, 22, []int{2}, nil)        // avg 2.0, 1 review

	for _, ep := range listCenterEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			centers := fetchCenters(t, r, ep)

			rated := centerByID(t, centers, 1)
			assert.Equal(t, 4.0, rated.AvgRating, "%s must report the real average", ep.name)
			assert.Equal(t, int64(3), rated.ReviewCount, "%s must report the real count", ep.name)

			unrated := centerByID(t, centers, 2)
			assert.Equal(t, 0.0, unrated.AvgRating, "%s must report 0 for a centre with no reviews", ep.name)
			assert.Equal(t, int64(0), unrated.ReviewCount, "%s must report 0 for a centre with no reviews", ep.name)
		})
	}
}

// TestBeautyCenterListEndpoints_IncludeSchedulesSameShapeAsDetail — schedules
// must ride along in the list payload with the exact detail shape.
func TestBeautyCenterListEndpoints_IncludeSchedulesSameShapeAsDetail(t *testing.T) {
	r, db, _ := beautyCenterListTestRouter(t)
	blocks := [][3]any{{1, "09:00", "18:00"}, {2, "10:00", "17:00"}}
	seedListCenter(t, db, 1, "Scheduled Center", 10, 20, []int{5}, blocks)
	seedListCenter(t, db, 2, "Unscheduled Center", 11, 21, nil, nil)

	detailW := do(r, http.MethodGet, "/api/beauty-centers/1", "", "")
	require.Equal(t, http.StatusOK, detailW.Code, "detail body=%s", detailW.Body.String())
	var detail response.BeautyCenterResponse
	require.NoError(t, json.Unmarshal(detailW.Body.Bytes(), &detail))
	require.Len(t, detail.Schedules, 2, "detail is the reference shape")

	for _, ep := range listCenterEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			centers := fetchCenters(t, r, ep)
			got := centerByID(t, centers, 1)

			assert.ElementsMatch(t, detail.Schedules, got.Schedules,
				"%s schedules must match the detail payload", ep.name)

			// A centre with no working hours must stay null (same as detail does).
			unrated := centerByID(t, centers, 2)
			assert.Empty(t, unrated.Schedules)

			// Inner field names are the client contract.
			raw := rawCenterObject(t, r, ep, 1)
			schedules, ok := raw["schedules"].([]any)
			require.True(t, ok, "%s must expose a schedules array, got %T", ep.name, raw["schedules"])
			require.Len(t, schedules, 2)
			first := schedules[0].(map[string]any)
			for _, key := range []string{"id", "day_of_week", "start_time", "end_time"} {
				assert.Contains(t, first, key, "%s schedule is missing %q", ep.name, key)
			}
		})
	}
}

// rawCenterObject fetches the endpoint again and returns centre id's raw JSON
// object, so wire-level field names can be asserted.
func rawCenterObject(t *testing.T, r *gin.Engine, ep listEndpoint, id uint) map[string]any {
	t.Helper()
	w := do(r, http.MethodGet, ep.path, "", "")
	require.Equal(t, http.StatusOK, w.Code, "%s body=%s", ep.name, w.Body.String())

	var rows []json.RawMessage
	if ep.paginated {
		var env struct {
			Data []json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
		rows = env.Data
	} else {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rows))
	}
	for _, row := range rows {
		var obj map[string]any
		require.NoError(t, json.Unmarshal(row, &obj))
		if uint(obj["id"].(float64)) == id {
			return obj
		}
	}
	t.Fatalf("centre %d not found in %s", id, ep.name)
	return nil
}

// TestBeautyCenterListEndpoints_NoNPlusOne — the rating aggregation and the
// schedules load must be batched: the SQL statement count must NOT grow with the
// number of centres on the page, and the reviews table must be aggregated once.
func TestBeautyCenterListEndpoints_NoNPlusOne(t *testing.T) {
	r, db, rec := beautyCenterListTestRouter(t)
	seedListCenter(t, db, 1, "Center One", 10, 20, []int{5, 4}, [][3]any{{1, "09:00", "18:00"}})

	rec.reset()
	small := fetchCenters(t, r, listEndpoint{"list", "/api/beauty-centers?page=1&limit=10", true})
	require.Len(t, small, 1)
	queriesForOne := rec.count()

	for id := uint(2); id <= 8; id++ {
		seedListCenter(t, db, id, fmt.Sprintf("Center %d", id), 10+float64(id), 20+float64(id), []int{3}, [][3]any{{1, "09:00", "18:00"}})
	}

	rec.reset()
	large := fetchCenters(t, r, listEndpoint{"list", "/api/beauty-centers?page=1&limit=10", true})
	require.Len(t, large, 8)
	queriesForMany := rec.count()

	assert.Equal(t, queriesForOne, queriesForMany,
		"query count must not grow with the page size (got %d for 1 centre, %d for 8)", queriesForOne, queriesForMany)
	assert.LessOrEqual(t, queriesForMany, 5,
		"list must stay within a fixed small query budget, got %d", queriesForMany)
	assert.Equal(t, 1, strings.Count(rec.all(), "reviews"),
		"the reviews table must be aggregated exactly once for the whole page")
	t.Logf("list query budget: %d statements for 1 centre, %d for 8 centres; review aggregates: %d",
		queriesForOne, queriesForMany, strings.Count(rec.all(), "reviews"))
	t.Logf("statements:\n%s", rec.all())
}

// TestBeautyCenterListEndpoints_PaginationStillWorks — the fix must not disturb
// offset/limit paging or the active-only total.
func TestBeautyCenterListEndpoints_PaginationStillWorks(t *testing.T) {
	r, db, _ := beautyCenterListTestRouter(t)
	seedListCenter(t, db, 1, "Page Center A", 10, 20, []int{5}, nil)
	seedListCenter(t, db, 2, "Page Center B", 11, 21, []int{4}, nil)
	seedListCenter(t, db, 3, "Page Center C", 12, 22, []int{3}, nil)
	seedListCenter(t, db, 4, "Inactive Center", 13, 23, nil, nil)
	require.NoError(t, db.Model(&models.BeautyCenter{}).Where("id = ?", 4).Update("is_active", false).Error)

	w := do(r, http.MethodGet, "/api/beauty-centers?page=1&limit=2", "", "")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var first struct {
		Data  []response.BeautyCenterResponse `json:"data"`
		Total int64                           `json:"total"`
		Page  int                             `json:"page"`
		Limit int                             `json:"limit"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &first))
	assert.Len(t, first.Data, 2)
	assert.Equal(t, int64(3), first.Total, "inactive centres must not be counted")
	assert.Equal(t, 1, first.Page)
	assert.Equal(t, 2, first.Limit)

	w = do(r, http.MethodGet, "/api/beauty-centers?page=2&limit=2", "", "")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var second struct {
		Data []response.BeautyCenterResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &second))
	assert.Len(t, second.Data, 1)

	ids := []uint{first.Data[0].ID, first.Data[1].ID, second.Data[0].ID}
	assert.ElementsMatch(t, []uint{1, 2, 3}, ids, "pages must not repeat or drop centres")
}

// TestBeautyCenterDetailAndRanking_Unchanged — the detail and ranking payloads
// keep their shape and now agree on the aggregate values.
func TestBeautyCenterDetailAndRanking_Unchanged(t *testing.T) {
	r, db, _ := beautyCenterListTestRouter(t)
	seedListCenter(t, db, 1, "Kami Center", 10, 20, []int{5, 4, 3}, [][3]any{{1, "09:00", "18:00"}})
	seedListCenter(t, db, 2, "Bella Center", 11, 21, []int{2}, nil)

	t.Run("detail carries the aggregate and schedules", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/beauty-centers/1", "", "")
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		var detail response.BeautyCenterResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &detail))
		assert.Equal(t, 4.0, detail.AvgRating)
		assert.Equal(t, int64(3), detail.ReviewCount)
		assert.Len(t, detail.Schedules, 1)
	})

	t.Run("ranking still reports the same aggregate", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/beauty-centers/ranking?period=all&limit=10", "", "")
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		var ranking []response.RankingResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ranking))
		require.NotEmpty(t, ranking)

		var kami *response.RankingResponse
		for i := range ranking {
			if ranking[i].CenterID == 1 {
				kami = &ranking[i]
			}
		}
		require.NotNil(t, kami, "ranking must include centre 1")
		assert.Equal(t, 4.0, kami.AvgRating)
		assert.Equal(t, int64(3), kami.ReviewCount)
	})
}

// TestBeautyCenterAggregateFields_AreReadOnlyProjections — avg_rating and
// review_count are computed, not stored: loading a centre populates them, but
// saving it must not attempt to write columns that do not exist.
func TestBeautyCenterAggregateFields_AreReadOnlyProjections(t *testing.T) {
	_, db, _ := beautyCenterListTestRouter(t)
	seedListCenter(t, db, 1, "Writable Center", 10, 20, []int{5, 4, 3}, [][3]any{{1, "09:00", "18:00"}})

	repo := repository.NewBeautyCenterRepositoryImpl(db)
	center, err := repo.FindByID(1)
	require.NoError(t, err)
	require.Equal(t, 4.0, center.AvgRating)
	require.Equal(t, int64(3), center.ReviewCount)

	center.Name = "Renamed Center"
	require.NoError(t, repo.Update(center), "saving a loaded centre must not touch computed columns")

	var reloaded models.BeautyCenter
	require.NoError(t, db.First(&reloaded, 1).Error)
	assert.Equal(t, "Renamed Center", reloaded.Name)

	assert.False(t, db.Migrator().HasColumn(&models.BeautyCenter{}, "avg_rating"),
		"avg_rating must stay a projection, never a schema column")
	assert.False(t, db.Migrator().HasColumn(&models.BeautyCenter{}, "review_count"),
		"review_count must stay a projection, never a schema column")
}
