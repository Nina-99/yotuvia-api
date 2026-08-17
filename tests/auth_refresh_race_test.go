package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/router"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newAuthRaceDB uses a named shared-cache in-memory database with a single
// connection, the same setup the token-ledger concurrency tests use: the two
// goroutines serialize on one connection exactly like Postgres row locks
// serialize on FOR UPDATE (design D2). The DSN is unique per test invocation
// (timestamp suffix) and the pool is closed at cleanup so repeated runs
// (-count, parallel tests) never share stale session state.
func newAuthRaceDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open race db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, m := range authRefreshModels() {
		if err := db.AutoMigrate(m); err != nil {
			t.Fatalf("failed to migrate %T: %v", m, err)
		}
	}
	return db
}

// TestAuthRefresh_ConcurrentRotationSingleWinner — tasks 1.9: two parallel
// refreshes presenting the SAME refresh token must produce exactly one 200;
// the loser gets 401 and the whole family ends up revoked (D2 race semantics:
// the loser treats the stale token as the theft signal and revokes the family).
func TestAuthRefresh_ConcurrentRotationSingleWinner(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := newAuthRaceDB(t)
	validate := validator.New()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api")
	router.AuthAccountRoutes(rg, db, validate)

	// Seed one client + one active session generation with a known token so the
	// race is fully deterministic (no entropy in the critical path).
	client := models.Client{
		FirstName: "Race", LastName: "User", Username: "racer",
		Phone: "5550099", Email: "race@test.com", Password: "x", IsActive: true,
	}
	assert.NoError(t, db.Create(&client).Error)

	raw := "race-token-raw-1234567890"
	family := "family-race-1"
	assert.NoError(t, db.Create(&models.UserSession{
		UserID:           client.ID,
		FamilyID:         family,
		RefreshTokenHash: utils.HashRefreshToken(raw),
		ExpiresAt:        time.Now().Add(30 * 24 * time.Hour),
		Role:             "client",
	}).Error)

	reqBody, _ := json.Marshal(map[string]string{"refresh_token": raw})

	codes := make([]int, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(idx int) {
			defer wg.Done()
			w := do(r, http.MethodPost, "/api/auth-accounts/refresh", "", string(reqBody))
			codes[idx] = w.Code
		}(i)
	}
	wg.Wait()

	successes, unauthorized := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			successes++
		case http.StatusUnauthorized:
			unauthorized++
		}
	}
	assert.Equal(t, 1, successes, "exactly one concurrent refresh must succeed")
	assert.Equal(t, 1, unauthorized, "the losing refresh must get 401")

	// The race leaves exactly one rotated generation and revokes the family.
	var rows []models.UserSession
	assert.NoError(t, db.Where("family_id = ?", family).Find(&rows).Error)
	assert.Len(t, rows, 2, "rotation must create a new generation row")
	for _, row := range rows {
		assert.True(t, row.IsRevoked, "family generation %d must be revoked after the race", row.ID)
	}
}
