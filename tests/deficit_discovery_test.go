package tests

// The deficit clock and the discovery grace window.
//
// A centre that runs out of real headroom (balance - holds + overdraft <= 0)
// cannot settle the commission of another visit. It keeps its profile and a
// grace period, then stops being DISCOVERABLE (search/ranking/nearby) while
// remaining reachable by id. These specs pin both halves: the clock's lifecycle
// and the filter that reads it.

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDeficitDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// :memory: SQLite is PER-CONNECTION: a pooled second connection would see an
	// empty database, and the NOT EXISTS filter would silently match nothing.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(
		&models.BeautyCenter{},
		&models.BeautyCenterImage{},
		&models.WorkingHour{},
		&models.Review{},
		&models.SocialMedia{},
		&models.TokenAccount{},
		&models.TokenTransaction{},
		&models.TokenTopUp{},
	))
	return db
}

// deficitService builds the minimum token service surface the clock needs.
func deficitService(t *testing.T, db *gorm.DB) (accountRepo repository.TokenAccountRepository) {
	t.Helper()
	return repository.NewTokenAccountRepositoryImpl(db)
}

// TestDeficitSince_Lifecycle — stamped exactly when headroom hits zero, cleared
// only by a top-up that restores a POSITIVE available balance.
func TestDeficitSince_Lifecycle(t *testing.T) {
	db := newDeficitDB(t)

	repo := repository.NewTokenAccountRepositoryImpl(db)

	// Premium tier: -30 Ytv of overdraft (30_000 milli), deterministic.
	require.NoError(t, db.Create(&models.TokenAccount{
		BeautyCenterID: 1, Balance: 1000, TotalPurchased: 500_000, Currency: "USD",
	}).Error)

	// Spend the real balance: headroom is still +30 Ytv of credit, so the clock
	// must NOT start yet — the centre can still fund visits with the overdraft.
	_, err := repo.Consume(t.Context(), 1, 1000, "consumption", "spend real funds", nil)
	require.NoError(t, err)

	got, err := repo.FindByBeautyCenterID(1)
	require.NoError(t, err)
	assert.Equal(t, int64(0), got.Balance)
	assert.Nil(t, got.DeficitSince, "credit headroom remains, so the grace clock must not run")

	// Now drain the overdraft to the floor.
	err = repo.Transaction(func(tx *gorm.DB) error {
		return repo.DebitInTx(tx, got.ID, 30_000, 30_000, "consumption", "drain to floor", nil)
	})
	require.NoError(t, err)

	got, err = repo.FindByBeautyCenterID(1)
	require.NoError(t, err)
	assert.Equal(t, int64(-30_000), got.Balance)
	require.NotNil(t, got.DeficitSince, "at the floor the clock must start")

	firstStamp := *got.DeficitSince

	// A top-up that only reduces the debt does NOT clear it: available is still
	// negative, so the centre still cannot fund a new visit.
	_, err = repo.AddTokens(t.Context(), got.ID, utils.ToMilli(10), "topup", "partial", nil)
	require.NoError(t, err)
	got, err = repo.FindByBeautyCenterID(1)
	require.NoError(t, err)
	assert.Equal(t, utils.ToMilli(-20), got.Balance)
	require.NotNil(t, got.DeficitSince, "still in debt: the clock must keep running")
	assert.Equal(t, firstStamp.Unix(), got.DeficitSince.Unix(), "and must not be restamped")

	// A top-up that puts the available balance positive ends the deficit.
	_, err = repo.AddTokens(t.Context(), got.ID, utils.ToMilli(25), "topup", "full", nil)
	require.NoError(t, err)
	got, err = repo.FindByBeautyCenterID(1)
	require.NoError(t, err)
	assert.Equal(t, utils.ToMilli(5), got.Balance)
	assert.Nil(t, got.DeficitSince, "positive available balance must clear the clock")
}

// TestDiscovery_HidesDeficitCentersPastGrace — the filter that makes a
// non-paying centre disappear from the listings, without deleting anything.
func TestDiscovery_HidesDeficitCentersPastGrace(t *testing.T) {
	db := newDeficitDB(t)
	t.Setenv("TOKEN_DEFICIT_GRACE_HOURS", "168") // 7 days

	healthy := models.BeautyCenter{Name: "Healthy", Email: "healthy@test.com", ReferralCode: "REFH", IsActive: true}
	longDeficit := models.BeautyCenter{Name: "Long deficit", Email: "long@test.com", ReferralCode: "REFL", IsActive: true}
	freshDeficit := models.BeautyCenter{Name: "Fresh deficit", Email: "fresh@test.com", ReferralCode: "REFF", IsActive: true}
	inactive := models.BeautyCenter{Name: "Inactive", Email: "inactive@test.com", ReferralCode: "REFI", IsActive: false}
	for _, c := range []*models.BeautyCenter{&healthy, &longDeficit, &freshDeficit, &inactive} {
		require.NoError(t, db.Create(c).Error)
	}
	// IsActive carries `default:true`, and GORM omits a zero-value field that has
	// a default on Create — so `IsActive: false` above was stored as TRUE. Flip it
	// explicitly, which is what the real admin flow does too.
	require.NoError(t, db.Model(&models.BeautyCenter{}).Where("id = ?", inactive.ID).
		Update("is_active", false).Error)

	longAgo := time.Now().Add(-10 * 24 * time.Hour)
	recent := time.Now().Add(-1 * time.Hour)
	require.NoError(t, db.Create(&models.TokenAccount{
		BeautyCenterID: longDeficit.ID, Balance: -30_000, DeficitSince: &longAgo, Currency: "USD",
	}).Error)
	require.NoError(t, db.Create(&models.TokenAccount{
		BeautyCenterID: freshDeficit.ID, Balance: -5_000, DeficitSince: &recent, Currency: "USD",
	}).Error)

	repo := repository.NewBeautyCenterRepositoryImpl(db)

	t.Run("listings exclude only the centre past the grace", func(t *testing.T) {
		centers, total, err := repo.FindAll(1, 20)
		require.NoError(t, err)

		names := map[string]bool{}
		for _, c := range centers {
			names[c.Name] = true
		}
		assert.True(t, names["Healthy"])
		assert.True(t, names["Fresh deficit"], "inside the grace a centre stays visible")
		assert.False(t, names["Long deficit"], "past the grace it disappears from discovery")
		assert.False(t, names["Inactive"])
		assert.Equal(t, int64(2), total, "the count must agree with the page")
	})

	// Search had the scope on the COUNT but not on the page query, so the data
	// and the total disagreed. This pins both sides of that contract.
	t.Run("search filters the page and the count together", func(t *testing.T) {
		centers, total, err := repo.Search("deficit", 1, 20)
		require.NoError(t, err)
		assert.Len(t, centers, int(total), "page size must agree with the total")
		for _, c := range centers {
			assert.NotEqual(t, "Long deficit", c.Name, "past the grace it is not searchable")
		}
		require.Len(t, centers, 1, "only the centre inside the grace remains")
		assert.Equal(t, "Fresh deficit", centers[0].Name)
	})

	t.Run("its profile is still reachable by id", func(t *testing.T) {
		center, err := repo.FindByID(longDeficit.ID)
		require.NoError(t, err)
		require.NotNil(t, center, "a client with a link, or a booking already made, must still see it")
		assert.Equal(t, "Long deficit", center.Name)
	})

	t.Run("the grace is configurable", func(t *testing.T) {
		t.Setenv("TOKEN_DEFICIT_GRACE_HOURS", "0")
		centers, _, err := repo.FindAll(1, 20)
		require.NoError(t, err)
		for _, c := range centers {
			assert.NotEqual(t, "Fresh deficit", c.Name, "a zero grace hides any deficit immediately")
		}
	})
}
