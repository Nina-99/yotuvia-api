package service

// The booking solvency gate: a reservation must not be created when the centre
// cannot settle its commission, because the charge happens at COMPLETION and a
// booking the centre can never close is a promise broken days later.
//
// The two outcomes are deliberately distinct and both neutral to a client:
//   - inactive centre            -> ErrCenterInactive
//   - cannot fund the commission -> ErrCenterUnavailable
//
// The centre's own booking path wraps them with actionable text.

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSolvencySvc(t *testing.T, balanceMilli int64) (*ReservationServiceImpl, *gorm.DB, uint) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// :memory: is per-connection; a pooled second connection sees an empty DB.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	// BeautyCenterRepository.FindByID joins reviews and preloads the centre's
	// relations, so those tables must exist even though this test never reads them.
	require.NoError(t, db.AutoMigrate(
		&models.BeautyCenter{}, &models.BeautyCenterImage{}, &models.WorkingHour{},
		&models.SocialMedia{}, &models.Review{},
		&models.TokenAccount{}, &models.TokenTopUp{}, &models.TokenTransaction{},
	))
	center := models.BeautyCenter{Name: "C", Email: "c@test.com", ReferralCode: "C1", IsActive: true}
	require.NoError(t, db.Create(&center).Error)

	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	require.NoError(t, db.Create(&models.TokenAccount{
		BeautyCenterID: center.ID, Balance: balanceMilli, Currency: "USD",
	}).Error)

	tokenSvc := NewTokenAccountServiceImpl(
		accountRepo,
		repository.NewTokenTopUpRepositoryImpl(db),
		centerRepo,
		nil, // missions are irrelevant to the balance read
		nil, // notifications are irrelevant to the balance read
	)

	// MarketConfigService left nil on purpose: the helper falls back to
	// defaultEconomy (3.5% with a 0.35 Ytv floor), which is what we assert on.
	return &ReservationServiceImpl{
		CenterRepository:    centerRepo,
		TokenAccountService: tokenSvc,
	}, db, center.ID
}

// TestEnsureCenterCanFundBooking_EmptyWallet — the gate must use the EXACT
// charge for the cart, not a flat estimate: a centre at the floor can still
// serve a cheap service but not an expensive one.
func TestEnsureCenterCanFundBooking_EmptyWallet(t *testing.T) {
	svc, _, centerID := newSolvencySvc(t, 0) // new centre: 5 Ytv of overdraft

	// 3.5% of 100 = 3.5 Ytv, inside the overdraft -> allowed.
	assert.NoError(t, svc.ensureCenterCanFundBooking(centerID, 100),
		"a cheap cart is fundable with the overdraft")

	// 3.5% of 1000 = 35 Ytv, beyond the 5 Ytv overdraft -> refused.
	assert.ErrorIs(t, svc.ensureCenterCanFundBooking(centerID, 1000),
		apperrors.ErrCenterUnavailable)
}

// TestEnsureCenterCanFundBooking_FundedCenter — real funds extend the reach.
func TestEnsureCenterCanFundBooking_FundedCenter(t *testing.T) {
	// 100 Ytv balance + 5 Ytv overdraft = 105 Ytv available.
	svc, _, centerID := newSolvencySvc(t, utils.ToMilli(100))

	// 3.5% of 2000 = 70 Ytv -> covered.
	assert.NoError(t, svc.ensureCenterCanFundBooking(centerID, 2000))

	// 3.5% of 5000 = 175 Ytv -> not covered even with the overdraft.
	assert.ErrorIs(t, svc.ensureCenterCanFundBooking(centerID, 5000),
		apperrors.ErrCenterUnavailable)
}

func TestEnsureCenterCanFundBooking_InactiveCenter(t *testing.T) {
	svc, db, centerID := newSolvencySvc(t, utils.ToMilli(1000))
	// `IsActive` carries `default:true`, so an explicit false at Create is
	// written as true. Deactivate the way the admin flow does.
	require.NoError(t, db.Model(&models.BeautyCenter{}).Where("id = ?", centerID).
		Update("is_active", false).Error)

	assert.ErrorIs(t, svc.ensureCenterCanFundBooking(centerID, 100),
		apperrors.ErrCenterInactive)
}

// TestEnsureCenterCanFundBooking_SuspendedCentre — suspension is the platform's
// decision (deficit outlived the window), distinct from is_active, and it has
// its own outcome so the owner gets the right copy.
func TestEnsureCenterCanFundBooking_SuspendedCentre(t *testing.T) {
	svc, db, centerID := newSolvencySvc(t, utils.ToMilli(1000))
	now := time.Now()
	require.NoError(t, db.Model(&models.TokenAccount{}).
		Where("beauty_center_id = ?", centerID).
		Updates(map[string]interface{}{"suspended_at": now}).Error)

	assert.ErrorIs(t, svc.ensureCenterCanFundBooking(centerID, 100),
		apperrors.ErrCenterSuspended)
}

// TestEnsureCenterCanFundBooking_NoEconomyWiring — embeddings and test routers
// without the token service keep the historical behaviour instead of refusing
// every booking.
func TestEnsureCenterCanFundBooking_NoEconomyWiring(t *testing.T) {
	svc, _, centerID := newSolvencySvc(t, 0)
	svc.TokenAccountService = nil

	assert.NoError(t, svc.ensureCenterCanFundBooking(centerID, 999_999))
}
