package tests

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────
//  Slice 1 (feat-loyalty-client-experience): visible earn/redeem.
//  Nullable duration/expires_at ride along UNENFORCED; apply_loyalty
//  nil/absent = legacy auto-apply; UNIQUE(reservation, card) guards
//  double-punch; ONE notification per verification.
// ─────────────────────────────────────────────────────────────

// slice1LoyaltyService builds the REAL loyalty service over SQLite.
func slice1LoyaltyService(t *testing.T, db *gorm.DB) service.LoyaltyService {
	t.Helper()
	return service.NewLoyaltyService(
		repository.NewLoyaltyRepositoryImpl(db),
		repository.NewBeautyCenterRepositoryImpl(db),
		nil,
		validator.New(),
	)
}

func seedSlice1Program(t *testing.T, db *gorm.DB, id, centerID, serviceID uint, active bool) {
	t.Helper()
	svcID := serviceID
	require.NoError(t, db.Create(&models.LoyaltyProgram{
		Model:          gorm.Model{ID: id},
		BeautyCenterID: centerID,
		ServiceID:      &svcID,
		RequiredSteps:  5,
		RewardType:     "free",
		IsActive:       true,
	}).Error)
	if !active {
		// NOTE: the model carries gorm:"default:true", so an explicit false on
		// Create is overwritten by the DB default — flip it afterwards.
		require.NoError(t, db.Model(&models.LoyaltyProgram{}).Where("id = ?", id).Update("is_active", false).Error)
	}
}

// TestLoyaltyHistory_Authz — GET /loyalty/client/:id/history: owner reads its
// timeline, cross-client is 403 with no leak, anonymous is 401.
func TestLoyaltyHistory_Authz(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := loyaltyTestRouter(t)
	seedLoyaltyFixtures(t, db)
	// One punch row on card 1 so the timeline is non-empty.
	require.NoError(t, db.Create(&models.PunchHistory{
		CustomerPunchCardID: 1,
		ReservationID:       42,
	}).Error)

	t.Run("owner reads own history", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/loyalty/client/1/history", roleToken(t, 1, 0, "client"), "")
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), `"card_id":1`)
	})

	t.Run("cross-client history is 403 without leak", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/loyalty/client/1/history", roleToken(t, 9, 0, "client"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.NotContains(t, w.Body.String(), "card_id")
	})

	t.Run("anonymous history is 401", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/loyalty/client/1/history", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("owner reads pending rewards", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/loyalty/client/1/pending", roleToken(t, 1, 0, "client"), "")
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	})

	t.Run("cross-client pending is 403", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/loyalty/client/1/pending", roleToken(t, 9, 0, "client"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

// TestLoyaltyPublicPrograms_Anonymous — the NEW public route serves active
// programs without a token; the admin-only center route stays gated.
func TestLoyaltyPublicPrograms_Anonymous(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := loyaltyTestRouter(t)
	seedLoyaltyFixtures(t, db)
	seedSlice1Program(t, db, 9, 1, 1, false) // inactive: must not leak

	t.Run("anonymous public programs are served", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/loyalty/public/1", "", "")
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), `"required_steps":5`)
	})

	t.Run("inactive programs are hidden from public", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/loyalty/public/1", "", "")
		assert.Equal(t, http.StatusOK, w.Code)
		// Only program 1 (active) is listed — exactly one entry.
		assert.Equal(t, 1, countJSONOccurrences(w.Body.String(), `"required_steps"`))
	})

	t.Run("admin center route still requires beauty_center", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/loyalty/center/1", roleToken(t, 1, 0, "client"), "")
		assert.Equal(t, http.StatusForbidden, w.Code,
			"the admin route must NOT have been widened by the public route")
	})
}

// countJSONOccurrences counts raw substring occurrences (good enough for tiny bodies).
func countJSONOccurrences(body, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(body); i++ {
		if body[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

// TestLoyaltyDoublePunch_UniqueGuard — two AddPunch calls for the same
// (reservation, card) mint exactly ONE punch row and ONE stamp.
func TestLoyaltyDoublePunch_UniqueGuard(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedDashboardCenter(t, db, 1, "Punch Center", "punch@test.com", true)
	seedDashboardService(t, db, 1, 1, "Svc", true)
	seedDashboardClient(t, db, 1, "Punch", "Client")
	seedSlice1Program(t, db, 1, 1, 1, true)

	svc := slice1LoyaltyService(t, db)
	require.NoError(t, svc.AddPunch(77, 1, 1, 1))
	require.NoError(t, svc.AddPunch(77, 1, 1, 1), "second punch is a no-op success")

	var n int64
	require.NoError(t, db.Model(&models.PunchHistory{}).Where("reservation_id = ?", 77).Count(&n).Error)
	assert.Equal(t, int64(1), n, "UNIQUE(reservation, card) must hold exactly one row")

	var card models.CustomerPunchCard
	require.NoError(t, db.Where("client_id = ? AND loyalty_program_id = ?", 1, 1).First(&card).Error)
	assert.Equal(t, 1, card.CurrentSteps, "double-punch must not double-count stamps")
}

// TestLoyaltyDoublePunch_ConcurrentRace — concurrent punches against ONE
// pre-existing card converge on a single punch row (procedural guard + UNIQUE
// absorb the race). Card-creation races are gated upstream by
// CompleteVerificationTx (exactly-once scan), so the guard contract here is
// scoped to the (reservation, card) pair.
func TestLoyaltyDoublePunch_ConcurrentRace(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedDashboardCenter(t, db, 1, "Race Center", "race@test.com", true)
	seedDashboardService(t, db, 1, 1, "Svc", true)
	seedDashboardClient(t, db, 1, "Race", "Client")
	seedSlice1Program(t, db, 1, 1, 1, true)

	svc := slice1LoyaltyService(t, db)
	// Pre-create the card on an unrelated reservation so every racer shares it.
	require.NoError(t, svc.AddPunch(700, 1, 1, 1))
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = svc.AddPunch(78, 1, 1, 1)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		assert.NoError(t, err, "racer %d must succeed (or no-op)", i)
	}

	var n int64
	require.NoError(t, db.Model(&models.PunchHistory{}).Where("reservation_id = ?", 78).Count(&n).Error)
	assert.Equal(t, int64(1), n, "concurrent scans must converge on one punch row")
}

// TestLoyaltyDuration_NullableUnenforced — programs/cards persist duration and
// derived expires_at WITHOUT enforcement in slice 1.
func TestLoyaltyDuration_NullableUnenforced(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedDashboardCenter(t, db, 1, "Dur Center", "dur@test.com", true)
	seedDashboardService(t, db, 1, 1, "Svc", true)
	seedDashboardClient(t, db, 1, "Dur", "Client")

	svc := slice1LoyaltyService(t, db)
	days := 30
	// Program without duration → cards carry nil expires_at, punches still mint.
	seedSlice1Program(t, db, 1, 1, 1, true)
	require.NoError(t, svc.AddPunch(90, 1, 1, 1))
	var card models.CustomerPunchCard
	require.NoError(t, db.Where("client_id = ?", 1).First(&card).Error)
	assert.Nil(t, card.ExpiresAt, "no policy = no expiry, and the punch still mints")
	assert.Equal(t, 1, card.CurrentSteps)

	// Program WITH duration → new cards derive expires_at ≈ now+days.
	require.NoError(t, db.Model(&models.LoyaltyProgram{}).Where("id = ?", 1).Update("duration_days", days).Error)
	require.NoError(t, db.Delete(&models.CustomerPunchCard{}, card.ID).Error)
	require.NoError(t, svc.AddPunch(91, 1, 1, 1))
	var card2 models.CustomerPunchCard
	require.NoError(t, db.Where("client_id = ?", 1).First(&card2).Error)
	require.NotNil(t, card2.ExpiresAt, "duration policy derives expires_at")
	assert.WithinDuration(t, time.Now().AddDate(0, 0, days), *card2.ExpiresAt, 5*time.Minute)
}

// TestReservationApplyLoyalty_Choice — nil/absent = legacy auto-apply (back-
// compat), explicit false = skip earn AND redeem, true = auto-apply.
func TestReservationApplyLoyalty_Choice(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := bookingAvailabilityRouter(t)
	seedDashboardCenter(t, db, 1, "Choice Center", "choice@test.com", true)
	require.NoError(t, db.Model(&models.BeautyCenter{}).Where("id = ?", 1).Update("loyalty_enabled", true).Error)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardClient(t, db, 1, "Choice", "Client")
	seedServiceRow(t, db, 1, 1, fixtureServiceDuration)
	seedSlice1Program(t, db, 1, 1, 1, true)
	// Completed card → pending redemption for the booking flow.
	require.NoError(t, db.Create(&models.CustomerPunchCard{
		ClientID: 1, LoyaltyProgramID: 1, CurrentSteps: 5, Status: "completed",
	}).Error)

	tomorrow := time.Now().Add(24 * time.Hour)
	day := tomorrow.Format("2006-01-02")
	seedWorkingHour(t, db, 1, int(tomorrow.Weekday()), "00:00", "23:59")
	token := roleToken(t, 1, 0, "client")

	redemptions := func() int64 {
		var n int64
		require.NoError(t, db.Model(&models.LoyaltyRedemption{}).Count(&n).Error)
		return n
	}
	book := func(start, flagJSON string) string {
		t.Helper()
		body := fmt.Sprintf(
			`{"beauty_center_id":1,"personnel_id":1,"service_ids":[1],"date":%q,"start_time":%q%s}`,
			day, start, flagJSON)
		w := do(r, http.MethodPost, "/api/reservation/", token, body)
		assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		return w.Body.String()
	}

	// 1. Explicit false FIRST (card still completed): no redemption recorded.
	book("10:00", `,"apply_loyalty":false`)
	assert.Equal(t, int64(0), redemptions(), "apply_loyalty=false must skip redeem")

	// 2. Absent flag (legacy): auto-applies the pending reward.
	book("12:00", "")
	assert.Equal(t, int64(1), redemptions(), "absent flag = legacy auto-apply")

	// 3. Explicit true: a FRESH completed card (the previous one was already
	// redeemed — one card redeems once), then auto-applies again.
	require.NoError(t, db.Create(&models.CustomerPunchCard{
		ClientID: 1, LoyaltyProgramID: 1, CurrentSteps: 5, Status: "completed",
	}).Error)
	book("14:00", `,"apply_loyalty":true`)
	assert.Equal(t, int64(2), redemptions(), "apply_loyalty=true auto-applies")
}
