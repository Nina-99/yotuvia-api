package tests

// The owner-facing alerts and the suspension that follows an unfixed deficit.
//
// Contract: ONE notification per episode. A centre that completes ten visits
// while low gets one warning; a centre that recovers and dips again gets a new
// one; a centre that tops up has its suspension lifted automatically.

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAlertsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(
		&models.BeautyCenter{}, &models.TokenAccount{}, &models.TokenTopUp{},
		&models.TokenTransaction{}, &models.Notification{}, &models.PushDevice{},
	))
	return db
}

func newAlertsSvc(t *testing.T, db *gorm.DB, balanceMilli int64, deficitSince *time.Time) service.TokenAccountService {
	t.Helper()

	center := models.BeautyCenter{Name: "C", Email: "c@test.com", ReferralCode: "C1", IsActive: true}
	require.NoError(t, db.Create(&center).Error)
	require.NoError(t, db.Create(&models.TokenAccount{
		BeautyCenterID: center.ID,
		Balance:        balanceMilli,
		DeficitSince:   deficitSince,
		Currency:       "USD",
	}).Error)

	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	notifSvc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)
	return service.NewTokenAccountServiceImpl(
		accountRepo,
		repository.NewTokenTopUpRepositoryImpl(db),
		repository.NewBeautyCenterRepositoryImpl(db),
		nil,
		notifSvc,
	)
}

func countNotifications(t *testing.T, db *gorm.DB, notifType string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&models.Notification{}).Where("type = ?", notifType).Count(&n).Error)
	return n
}

// TestBalanceAlert_LowWarnsOncePerEpisode — the core dedupe guarantee.
func TestBalanceAlert_LowWarnsOncePerEpisode(t *testing.T) {
	db := newAlertsDB(t)
	// 5 Ytv available (new-centre overdraft included): under the 10 Ytv warning.
	svc := newAlertsSvc(t, db, 0, nil)

	require.NoError(t, svc.EvaluateBalanceAlerts(1))
	assert.Equal(t, int64(1), countNotifications(t, db, "token_low_balance"))

	// Ten more evaluations (one per completed visit) must NOT spam the owner.
	for i := 0; i < 10; i++ {
		require.NoError(t, svc.EvaluateBalanceAlerts(1))
	}
	assert.Equal(t, int64(1), countNotifications(t, db, "token_low_balance"),
		"one warning per episode, not per visit")

	// A top-up that clears the threshold resets the stamp...
	require.NoError(t, svc.AddTokens(1, utils.ToMilli(500), "topup", 1, "big top-up"))
	var refreshed models.TokenAccount
	require.NoError(t, db.First(&refreshed, "beauty_center_id = ?", 1).Error)
	assert.Nil(t, refreshed.LowBalanceNotifiedAt, "recovery clears the stamp")
}

// TestBalanceAlert_DeficitAndSuspension — the escalation, with the window
// configurable so the test does not wait 30 days.
func TestBalanceAlert_DeficitAndSuspension(t *testing.T) {
	t.Setenv("TOKEN_SUSPEND_AFTER_HOURS", "24")

	db := newAlertsDB(t)
	startedAt := time.Now().Add(-2 * time.Hour) // inside the window
	svc := newAlertsSvc(t, db, 0, &startedAt)

	require.NoError(t, svc.EvaluateBalanceAlerts(1))
	assert.Equal(t, int64(1), countNotifications(t, db, "token_deficit"))
	assert.Equal(t, int64(0), countNotifications(t, db, "token_suspended"),
		"inside the window there is no suspension yet")

	var beforeWindow models.TokenAccount
	require.NoError(t, db.First(&beforeWindow, "beauty_center_id = ?", 1).Error)
	assert.Nil(t, beforeWindow.SuspendedAt)

	// Time passes: the deficit outlives the window.
	old := time.Now().Add(-48 * time.Hour)
	require.NoError(t, db.Model(&models.TokenAccount{}).
		Where("beauty_center_id = ?", 1).Update("deficit_since", old).Error)

	require.NoError(t, svc.EvaluateBalanceAlerts(1))
	assert.Equal(t, int64(1), countNotifications(t, db, "token_suspended"))

	var suspendedRow models.TokenAccount
	require.NoError(t, db.First(&suspendedRow, "beauty_center_id = ?", 1).Error)
	require.NotNil(t, suspendedRow.SuspendedAt, "the centre must be suspended")

	suspended, err := svc.IsSuspended(1)
	require.NoError(t, err)
	assert.True(t, suspended)

	// Paying lifts the suspension automatically.
	require.NoError(t, svc.AddTokens(1, utils.ToMilli(100), "topup", 2, "recarga"))
	require.NoError(t, svc.EvaluateBalanceAlerts(1), "re-evaluating after the credit must succeed")
	var afterTopUp models.TokenAccount
	require.NoError(t, db.First(&afterTopUp, "beauty_center_id = ?", 1).Error)
	assert.Nil(t, afterTopUp.SuspendedAt, "a top-up must lift the suspension")
	assert.Nil(t, afterTopUp.DeficitNotifiedAt, "and reset the deficit episode")
}

// TestBalanceAlert_HealthyCenterIsSilent
func TestBalanceAlert_HealthyCenterIsSilent(t *testing.T) {
	db := newAlertsDB(t)
	svc := newAlertsSvc(t, db, utils.ToMilli(500), nil)

	require.NoError(t, svc.EvaluateBalanceAlerts(1))

	assert.Zero(t, countNotifications(t, db, "token_low_balance"))
	assert.Zero(t, countNotifications(t, db, "token_deficit"))
	assert.Zero(t, countNotifications(t, db, "token_suspended"))
}

// TestBalanceAlert_SweepCoversQuietCentres — the safety net for a centre that
// stopped transacting and would never hit an event-driven call site.
func TestBalanceAlert_SweepCoversQuietCentres(t *testing.T) {
	db := newAlertsDB(t)
	healthy := models.BeautyCenter{Name: "Healthy", Email: "h@t.com", ReferralCode: "H", IsActive: true}
	low := models.BeautyCenter{Name: "Low", Email: "l@t.com", ReferralCode: "L", IsActive: true}
	require.NoError(t, db.Create(&healthy).Error)
	require.NoError(t, db.Create(&low).Error)

	require.NoError(t, db.Create(&models.TokenAccount{
		BeautyCenterID: healthy.ID, Balance: utils.ToMilli(1000), Currency: "USD",
	}).Error)
	require.NoError(t, db.Create(&models.TokenAccount{
		BeautyCenterID: low.ID, Balance: 0, Currency: "USD",
	}).Error)

	svc := newAlertsSvcForDB(t, db)
	n, err := svc.SweepBalanceAlerts(time.Now())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1)

	assert.Equal(t, int64(1), countNotifications(t, db, "token_low_balance"),
		"the quiet low-balance centre must be warned by the sweep")
}

// newAlertsSvcForDB reuses existing rows (the helper above seeds its own centre).
func newAlertsSvcForDB(t *testing.T, db *gorm.DB) service.TokenAccountService {
	t.Helper()
	return service.NewTokenAccountServiceImpl(
		repository.NewTokenAccountRepositoryImpl(db),
		repository.NewTokenTopUpRepositoryImpl(db),
		repository.NewBeautyCenterRepositoryImpl(db),
		nil,
		service.NewNotificationServiceImpl(
			repository.NewNotificationRepositoryImpl(db),
			repository.NewPushDeviceRepositoryImpl(db),
		),
	)
}

// TestTokenHealthSnapshot_States — the dashboard banner and the booking gate
// read the SAME source, so they can never disagree: "low" still books, the other
// two do not.
func TestTokenHealthSnapshot_States(t *testing.T) {
	t.Setenv("TOKEN_LOW_BALANCE_YTV", "10")

	cases := []struct {
		name        string
		balance     int64
		deficit     *time.Time
		suspended   *time.Time
		wantState   string
		wantCanBook bool
		wantDays    int
	}{
		{name: "healthy", balance: utils.ToMilli(500), wantState: "ok", wantCanBook: true},
		{name: "low but bookable", balance: 0, wantState: "low", wantCanBook: true},
		{
			name: "deficit blocks bookings", balance: 0,
			deficit:   ptrTime(time.Now().Add(-72 * time.Hour)),
			wantState: "deficit", wantCanBook: false, wantDays: 3,
		},
		{
			name: "suspended wins over deficit", balance: 0,
			deficit:   ptrTime(time.Now().Add(-72 * time.Hour)),
			suspended: ptrTime(time.Now()),
			wantState: "suspended", wantCanBook: false, wantDays: 3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newAlertsDB(t)
			svc := newAlertsSvc(t, db, tc.balance, tc.deficit)
			if tc.suspended != nil {
				require.NoError(t, db.Model(&models.TokenAccount{}).
					Where("beauty_center_id = ?", 1).
					Updates(map[string]interface{}{"suspended_at": *tc.suspended}).Error)
			}

			health, err := svc.HealthSnapshot(1)
			require.NoError(t, err)
			assert.Equal(t, tc.wantState, health.State)
			assert.Equal(t, tc.wantCanBook, health.CanAcceptBookings)
			if tc.wantDays > 0 {
				assert.InDelta(t, tc.wantDays, health.DeficitDays, 1)
			}
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
