package tests

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/configs"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/go-playground/validator/v10"
)

// ──────────────────────────────────────────────
//  feat-token-economy-config — market config (Unit 1)
// ──────────────────────────────────────────────

func newMarketConfigSUT(t *testing.T) (service.MarketConfigService, *gorm.DB) {
	t.Helper()
	db := setupRouterAuthzDB(t)
	repo := repository.NewMarketConfigRepositoryImpl(db)
	return service.NewMarketConfigServiceImpl(repo), db
}

// TestMarketConfig_FallbackOnMissing covers the safe-defaults path.
func TestMarketConfig_FallbackOnMissing(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, _ := newMarketConfigSUT(t)

	cfg := svc.GetEconomyForCurrency("USD")
	assert.Equal(t, 3.5, cfg.CommissionPct)
	assert.Equal(t, int64(350), cfg.MinChargeMilli)
	assert.Equal(t, 1.0, cfg.YottuToFiatRate)
}

// TestMarketConfig_HitReturnsRow covers the configured path.
func TestMarketConfig_HitReturnsRow(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, db := newMarketConfigSUT(t)

	assert.NoError(t, db.Create(&models.MarketConfig{
		Currency:        "BOB",
		YottuToFiatRate: 1.0,
		CommissionPct:   3.5,
		MinChargeMilli:  350,
	}).Error)

	cfg := svc.GetEconomyForCurrency("BOB")
	assert.Equal(t, 3.5, cfg.CommissionPct)
	assert.Equal(t, int64(350), cfg.MinChargeMilli)
}

// TestSeedMarketConfigs_IdempotentAndHealing verifies double-run creates one
// row and drifted values are restored to canonical.
func TestSeedMarketConfigs_IdempotentAndHealing(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	configs.DB = db
	defer func() { configs.DB = nil }()

	configs.SeedMarketConfigs()
	configs.SeedMarketConfigs()

	var rows []models.MarketConfig
	assert.NoError(t, db.Find(&rows).Error)
	if assert.Len(t, rows, 1) {
		assert.Equal(t, "BOB", rows[0].Currency)
		assert.Equal(t, 3.5, rows[0].CommissionPct)
		assert.Equal(t, int64(350), rows[0].MinChargeMilli)
	}

	drift := rows[0]
	drift.CommissionPct = 35
	drift.MinChargeMilli = 1
	assert.NoError(t, db.Save(&drift).Error)

	configs.SeedMarketConfigs()

	var healed models.MarketConfig
	assert.NoError(t, db.First(&healed).Error)
	assert.Equal(t, 3.5, healed.CommissionPct, "commission restored to canonical")
	assert.Equal(t, int64(350), healed.MinChargeMilli, "floor restored to canonical")
}

// ──────────────────────────────────────────────
//  feat-token-economy-config — Unit 2: atomic charge
// ──────────────────────────────────────────────

// TestComputeChargeMilli covers percentage above floor, floor on small
// tickets, and the exact boundary ticket.
func TestComputeChargeMilli(t *testing.T) {
	cases := []struct {
		name       string
		totalMilli int64
		pct        float64
		floorMilli int64
		wantMilli  int64
	}{
		{"percentage above floor", 200_000, 3.5, 350, 7000},
		{"floor applies below boundary", 5_000, 3.5, 350, 350},
		{"boundary ticket pays pct", 10_000, 3.5, 350, 350},
		{"zero total still charges floor", 0, 3.5, 350, 350},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := service.ComputeChargeMilli(tc.totalMilli, tc.pct, tc.floorMilli)
			assert.Equal(t, tc.wantMilli, got)
		})
	}
}

// newEconomyVerifySUT wires the FULL reservation service (real token +
// market config stacks) over a seeded center holding a token account.
func newEconomyVerifySUT(t *testing.T) (*gorm.DB, service.ReservationService) {
	t.Helper()
	db := setupRouterAuthzDB(t)
	seedDashboardCenter(t, db, 1, "Economy Center", "econ@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Svc", true)
	seedDashboardClient(t, db, 1, "Client", "One")

	account := &models.TokenAccount{BeautyCenterID: 1, Balance: 100_000, Currency: "BOB"} // 100 YOTTU
	if err := db.Create(account).Error; err != nil {
		t.Fatalf("failed to seed token account: %v", err)
	}

	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	marketRepo := repository.NewMarketConfigRepositoryImpl(db)
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	notifSvc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)

	// Two-phase wiring (same as reservation.router.go): mission svc needs
	// account svc for claims; account svc needs mission svc — break the cycle.
	missionSvc := service.NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	accountSvc := service.NewTokenAccountServiceImpl(accountRepo, topUpRepo, nil, missionSvc, notifSvc)
	missionSvc = service.NewTokenMissionServiceImpl(missionRepo, accountSvc, accountRepo)
	marketSvc := service.NewMarketConfigServiceImpl(marketRepo)
	// VerifyQR reaches LoyaltyService.GetCenterSettings on every successful
	// scan, so a nil here panics. Wire the real service (production does).
	loyaltySvc := service.NewLoyaltyService(
		repository.NewLoyaltyRepositoryImpl(db),
		repository.NewBeautyCenterRepositoryImpl(db),
		missionSvc,
		validator.New(),
	)

	reservationRepo := repository.NewReservationRepositoryImpl(db)
	svc := service.NewReservationServiceImpl(
		reservationRepo,
		repository.NewServiceRepositoryImpl(db),
		repository.NewPersonnelRepositoryImpl(db),
		repository.NewClientRepositoryImpl(db),
		accountRepo,
		notifSvc,
		loyaltySvc,
		nil, // PromotionService stub not needed for these flows
		accountSvc,
		missionSvc,
		marketSvc,
		validator.New(),
		repository.NewBeautyCenterRepositoryImpl(db),
		nil, // FeedPostRepository: attribution not exercised here
	)
	return db, svc
}

// seedVerifiableReservation inserts a PENDING reservation with a live QR token.
func seedVerifiableReservation(t *testing.T, db *gorm.DB, id uint, totalPrice float64) {
	t.Helper()
	token := "tok-" + strconv.FormatUint(uint64(id), 10)
	exp := utcMidnight(1).Add(12 * time.Hour)
	res := &models.Reservation{
		BeautyCenterID: 1, ClientID: 1, PersonnelID: 1,
		Date: utcMidnight(0), StartTime: "09:00", EndTime: "10:00",
		TotalPrice: totalPrice, Status: "PENDING",
		VerificationToken: &token, TokenExpiresAt: &exp,
	}
	res.ID = id
	if err := db.Create(res).Error; err != nil {
		t.Fatalf("failed to seed reservation %d: %v", id, err)
	}
}

func accountBalance(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var acc models.TokenAccount
	assert.NoError(t, db.Where("beauty_center_id = 1").First(&acc).Error)
	return acc.Balance
}

// TestEconomy_ChargeFollowsConfigAndFloor proves end-to-end: percentage for
// big tickets, floor for small ones, and live config changes honored.
func TestEconomy_ChargeFollowsConfigAndFloor(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	db, svc := newEconomyVerifySUT(t)

	assert.NoError(t, db.Create(&models.MarketConfig{
		Currency:        "BOB",
		YottuToFiatRate: 1.0,
		CommissionPct:   3.5,
		MinChargeMilli:  350,
	}).Error)

	// Big ticket: 200 Bs -> pct charge 7 YOTTU.
	seedVerifiableReservation(t, db, 101, 200)
	if _, _, err := svc.VerifyQR("tok-101", 1); err != nil {
		t.Fatalf("verify big ticket: %v", err)
	}
	assert.Equal(t, int64(93_000), accountBalance(t, db), "100 - 7 = 93 YOTTU")

	// Small ticket: 5 Bs -> floor 0.35 YOTTU.
	seedVerifiableReservation(t, db, 102, 5)
	if _, _, err := svc.VerifyQR("tok-102", 1); err != nil {
		t.Fatalf("verify small ticket: %v", err)
	}
	assert.Equal(t, int64(92_650), accountBalance(t, db), "93 - 0.35 = 92.65")

	// Ops changes config mid-life: commission 10%, floor 500 milli.
	assert.NoError(t, db.Model(&models.MarketConfig{}).Where("currency = ?", "BOB").
		Updates(map[string]interface{}{"commission_pct": 10.0, "min_charge_milli": 500}).Error)

	seedVerifiableReservation(t, db, 103, 200)
	if _, _, err := svc.VerifyQR("tok-103", 1); err != nil {
		t.Fatalf("verify after config change: %v", err)
	}
	assert.Equal(t, int64(72_650), accountBalance(t, db), "new pct applies: 92.65 - 20")
}

// TestEconomy_InsufficientRollsBackCompletion closes Fuga #1: when the debit
// guard rejects, the reservation is NOT completed and can be retried after
// recharging. No consumption ledger row is written.
func TestEconomy_InsufficientRollsBackCompletion(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	db, svc := newEconomyVerifySUT(t)

	// New-center overdraft is 5 YOTTU (5000 milli). Force balance down to
	// 1000 milli -> allowance 6000 milli. A 700 Bs ticket charges
	// 24500 milli > allowance -> guard must reject INSIDE the transaction.
	db.Model(&models.TokenAccount{}).Where("beauty_center_id = 1").
		Update("balance", 1_000)
	seedVerifiableReservation(t, db, 201, 700)

	_, _, err := svc.VerifyQR("tok-201", 1)
	assert.True(t, errors.Is(err, apperrors.ErrInsufficientBalance))

	var res models.Reservation
	assert.NoError(t, db.First(&res, 201).Error)
	assert.Equal(t, "PENDING", res.Status, "completion rolled back — retryable after recharge")

	var consumed int64
	assert.NoError(t, db.Model(&models.TokenTransaction{}).
		Where("beauty_center_id = 1 AND type = 'consumption'").Count(&consumed).Error)
	assert.Zero(t, consumed, "no ledger consumption row on rejected scan")
}

// TestEconomy_RescanNeverDoubleCharges preserves the legacy guarantee.
func TestEconomy_RescanNeverDoubleCharges(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	db, svc := newEconomyVerifySUT(t)

	seedVerifiableReservation(t, db, 202, 150) // charge 5.25 YOTTU

	if _, _, err := svc.VerifyQR("tok-202", 1); err != nil {
		t.Fatalf("first scan: %v", err)
	}
	first := accountBalance(t, db)

	// Second scan of the SAME token: token was cleared on completion ->
	// FindByToken misses -> ErrNotFound; balance must be untouched either way.
	_, _, _ = svc.VerifyQR("tok-202", 1)
	assert.Equal(t, first, accountBalance(t, db))
}
