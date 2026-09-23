package tests

import (
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// seedPersonnelMeReservations creates reservations for two personnel across
// several dates so scoping, date-range and ordering can be asserted:
//
//	staff 7: yesterday (-1d), +3d, +95d   ← the subject
//	staff 9: +1d                          ← must never leak into staff-7 results
func seedPersonnelMeReservations(t *testing.T, db *gorm.DB) {
	t.Helper()
	today := time.Now().Truncate(24 * time.Hour)
	rows := []models.Reservation{
		{BeautyCenterID: 1, ClientID: 1, PersonnelID: 7, Date: today.AddDate(0, 0, -1), StartTime: "09:00", EndTime: "10:00", Status: "CONFIRMED", TotalPrice: 100},
		{BeautyCenterID: 1, ClientID: 1, PersonnelID: 7, Date: today.AddDate(0, 0, 3), StartTime: "11:00", EndTime: "12:00", TotalPrice: 120},
		{BeautyCenterID: 1, ClientID: 1, PersonnelID: 7, Date: today.AddDate(0, 0, 95), StartTime: "14:00", EndTime: "15:00", Status: "PENDING", TotalPrice: 130},
		{BeautyCenterID: 1, ClientID: 2, PersonnelID: 9, Date: today.AddDate(0, 0, 1), StartTime: "16:00", EndTime: "17:00", TotalPrice: 140},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("failed to seed reservations: %v", err)
	}
}

func TestReservationRepo_FindByPersonnelId_ScopesToStaff(t *testing.T) {
	db := setupRouterAuthzDB(t)
	seedPersonnelMeReservations(t, db)
	repo := repository.NewReservationRepositoryImpl(db)

	res, err := repo.FindByPersonnelId(7, nil, nil)
	assert.NoError(t, err)
	assert.Len(t, res, 3, "staff 7 must see exactly its 3 reservations")

	// Cross-staff leak guard: staff 9's row must never appear.
	for _, r := range res {
		assert.Equal(t, uint(7), r.PersonnelID, "no cross-staff leak into staff 7 results")
	}
}

func TestReservationRepo_FindByPersonnelId_DateRange(t *testing.T) {
	db := setupRouterAuthzDB(t)
	seedPersonnelMeReservations(t, db)
	repo := repository.NewReservationRepositoryImpl(db)

	today := time.Now().Truncate(24 * time.Hour)
	from := today.AddDate(0, 0, -2)
	to := today.AddDate(0, 0, 2)

	res, err := repo.FindByPersonnelId(7, &from, &to)
	assert.NoError(t, err)
	assert.Len(t, res, 1, "only the -1d row fits inside [-2d, +2d]")
	assert.True(t, res[0].Date.Equal(today.AddDate(0, 0, -1)), "returns the -1d reservation")

	// Open-ended from: no lower bound → -1d AND +3d both included, +95d excluded.
	openEndedFrom := repoFindOpenTo(t, db, 7, &from, nil)
	assert.Len(t, openEndedFrom, 3, "nil to is open-ended")
}

// repoFindOpenTo calls FindByPersonnelId with a from bound and nil to.
func repoFindOpenTo(t *testing.T, db *gorm.DB, personnelID uint, from *time.Time, to *time.Time) []models.Reservation {
	t.Helper()
	repo := repository.NewReservationRepositoryImpl(db)
	res, err := repo.FindByPersonnelId(personnelID, from, to)
	assert.NoError(t, err)
	return res
}

func TestReservationRepo_FindByPersonnelId_OrderByDateAsc(t *testing.T) {
	db := setupRouterAuthzDB(t)
	seedPersonnelMeReservations(t, db)
	repo := repository.NewReservationRepositoryImpl(db)

	res, err := repo.FindByPersonnelId(7, nil, nil)
	assert.NoError(t, err)
	assert.Len(t, res, 3)

	// Ascending by date: -1d → +3d → +95d.
	assert.True(t, res[0].Date.Equal(time.Now().Truncate(24*time.Hour).AddDate(0, 0, -1)))
	assert.True(t, res[1].Date.Equal(time.Now().Truncate(24*time.Hour).AddDate(0, 0, 3)))
	assert.True(t, res[2].Date.Equal(time.Now().Truncate(24*time.Hour).AddDate(0, 0, 95)))
}

// TestReservationService_FindByPersonnelID — the service passes the claim
// scoping through to the repository; the who-is-the-actor decision stays in the
// controller (ctx clientID), so the service method carries no actor param.
func TestReservationService_FindByPersonnelID(t *testing.T) {
	db := setupRouterAuthzDB(t)
	seedPersonnelMeReservations(t, db)
	svc := newReservationServiceForMeTests(t, db)

	res, err := svc.FindByPersonnelID(7, nil, nil)
	assert.NoError(t, err)
	assert.Len(t, res, 3)
	for _, r := range res {
		assert.Equal(t, uint(7), r.PersonnelID)
	}

	today := time.Now().Truncate(24 * time.Hour)
	from := today.AddDate(0, 0, -2)
	to := today.AddDate(0, 0, 2)
	res, err = svc.FindByPersonnelID(7, &from, &to)
	assert.NoError(t, err)
	assert.Len(t, res, 1)
}

// newReservationServiceForMeTests builds a ReservationService wired with a real
// SQLite repository so agenda lookups hit the actual query path.
func newReservationServiceForMeTests(t *testing.T, db *gorm.DB) interface {
	FindByPersonnelID(personnelID uint, from, to *time.Time) ([]models.Reservation, error)
} {
	t.Helper()
	return service.NewReservationServiceImpl(
		repository.NewReservationRepositoryImpl(db),
		repository.NewServiceRepositoryImpl(db),
		repository.NewPersonnelRepositoryImpl(db),
		repository.NewClientRepositoryImpl(db),
		nil, // TokenAccountRepository not needed for these flows
		service.NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), repository.NewPushDeviceRepositoryImpl(db)),
		nil, // LoyaltyService stub not needed for reads
		nil, // PromotionService stub not needed for reads
		nil, // TokenAccountService stub not needed for reads
		nil, // TokenMissionService stub not needed for reads
		nil, // MarketConfigService not needed for reads
		validator.New(),
		repository.NewBeautyCenterRepositoryImpl(db),
	)
}
