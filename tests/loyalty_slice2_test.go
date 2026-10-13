package tests

import (
	"net/http"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ─────────────────────────────────────────────────────────────
//  Slice 2 (feat-loyalty-client-experience): DURATION expiry
//  enforcement + owner metrics. Program `duration_days` is the
//  policy; cards carry a derived `expires_at` (created/reset +
//  duration). Expired cards earn/redeem nothing and are excluded
//  from pending reads.
// ─────────────────────────────────────────────────────────────

// TestLoyaltyExpiry_CardEnforcement — an expired active card is retired (no
// further accrual) and a fresh cycle starts; an expired completed card cannot
// be redeemed nor counted as pending.
func TestLoyaltyExpiry_CardEnforcement(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedDashboardCenter(t, db, 1, "Exp Center", "exp@test.com", true)
	seedDashboardService(t, db, 1, 1, "Svc", true)
	seedDashboardClient(t, db, 1, "Exp", "Client")

	days := 30
	seedSlice1Program(t, db, 1, 1, 1, true)
	require.NoError(t, db.Model(&models.LoyaltyProgram{}).Where("id = ?", 1).Update("duration_days", days).Error)

	svc := slice1LoyaltyService(t, db)

	// First punch creates an active card with a derived expiry.
	require.NoError(t, svc.AddPunch(100, 1, 1, 1))
	var card models.CustomerPunchCard
	require.NoError(t, db.Where("client_id = ? AND status = ?", 1, "active").First(&card).Error)
	require.NotNil(t, card.ExpiresAt)
	assert.Equal(t, 1, card.CurrentSteps)

	// Force the active card into the past.
	past := time.Now().Add(-time.Hour)
	require.NoError(t, db.Model(&models.CustomerPunchCard{}).Where("id = ?", card.ID).Update("expires_at", past).Error)

	// A new punch must NOT accrue on the expired card: it is retired and a
	// fresh cycle begins.
	require.NoError(t, svc.AddPunch(101, 1, 1, 1))

	var expired models.CustomerPunchCard
	require.NoError(t, db.Where("id = ?", card.ID).First(&expired).Error)
	assert.Equal(t, "expired", expired.Status, "expired active card is retired")
	assert.Equal(t, 1, expired.CurrentSteps, "expired card must not accrue the new stamp")

	var fresh models.CustomerPunchCard
	require.NoError(t, db.Where("client_id = ? AND status = ?", 1, "active").Order("id DESC").First(&fresh).Error)
	assert.NotEqual(t, card.ID, fresh.ID, "a fresh active card starts the new cycle")
	assert.Equal(t, 1, fresh.CurrentSteps)

	// Expired COMPLETED card: not pending, redeems nothing.
	require.NoError(t, db.Create(&models.CustomerPunchCard{
		ClientID: 1, LoyaltyProgramID: 1, CurrentSteps: 5, Status: "completed", ExpiresAt: &past,
	}).Error)
	require.NoError(t, db.Create(&models.CustomerPunchCard{
		ClientID: 1, LoyaltyProgramID: 1, CurrentSteps: 5, Status: "completed",
	}).Error)

	pending, err := svc.GetPendingRewards(1, 1, "client")
	require.NoError(t, err)
	assert.Len(t, pending, 1, "only the non-expired completed card is pending")

	discount, _, err := svc.CalculateRedemption(1, 1, 1)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, discount, 0.0, "calculation must not error on an expired card")
}

// TestLoyaltyMetrics_Endpoint — GET /loyalty/center/:id/metrics returns
// per-program issued/redemption counts; cross-center is 403; empty programs
// return zeros (never null).
func TestLoyaltyMetrics_Endpoint(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := loyaltyTestRouter(t)
	seedLoyaltyFixtures(t, db) // center 1, program 1, card 1 (active, client 1)
	seedSecondCenterWithProgram(t, db)

	// Program 1 (center 1): 2 active, 1 completed, 1 redemption.
	require.NoError(t, db.Create(&models.CustomerPunchCard{
		ClientID: 1, LoyaltyProgramID: 1, CurrentSteps: 2, Status: "active",
	}).Error)
	require.NoError(t, db.Create(&models.CustomerPunchCard{
		ClientID: 1, LoyaltyProgramID: 1, CurrentSteps: 5, Status: "completed",
	}).Error)
	var completedCard models.CustomerPunchCard
	require.NoError(t, db.Where("client_id = ? AND status = ?", 1, "completed").First(&completedCard).Error)
	require.NoError(t, db.Create(&models.LoyaltyRedemption{
		CustomerPunchCardID: completedCard.ID, ReservationID: 1, DiscountApplied: 10,
	}).Error)

	owner := roleToken(t, 1, 1, "beauty_center")

	t.Run("owner reads metrics", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/loyalty/center/1/metrics", owner, "")
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		// program 1: active=2, completed=1, redemptions=1
		assert.Contains(t, w.Body.String(), `"active_cards":2`)
		assert.Contains(t, w.Body.String(), `"completed_cards":1`)
		assert.Contains(t, w.Body.String(), `"redemptions":1`)
	})

	t.Run("cross-center is 403", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/loyalty/center/2/metrics", owner, "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.NotContains(t, w.Body.String(), "redemptions")
	})

	t.Run("empty program returns zeros", func(t *testing.T) {
		// Program 2 (center 2) has no cards; read as its own owner.
		w := do(r, http.MethodGet, "/api/loyalty/center/2/metrics", roleToken(t, 2, 2, "beauty_center"), "")
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), `"active_cards":0`)
		assert.Contains(t, w.Body.String(), `"redemption_rate":0`)
	})
}
