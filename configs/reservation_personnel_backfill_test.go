package configs

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// migrateReservationPersonnelSchema installs the two tables the backfill reads.
func migrateReservationPersonnelSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&models.Reservation{}, &models.ReservationService{}))
}

// TestBackfillReservationServicePersonnel_FillsAndIsIdempotent pins the two
// acceptance properties: legacy NULL lines are filled from the reservation's
// lead, and a second run changes nothing.
func TestBackfillReservationServicePersonnel_FillsAndIsIdempotent(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := newSQLiteDB(t)
	migrateReservationPersonnelSchema(t, db)
	DB = db

	res := &models.Reservation{
		BeautyCenterID: 1,
		ClientID:       1,
		PersonnelID:    7,
		Date:           time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
		StartTime:      "10:00",
		EndTime:        "11:00",
		TotalPrice:     100,
		Status:         "PENDING",
	}
	require.NoError(t, db.Create(res).Error)

	legacy := &models.ReservationService{
		ReservationID: res.ID,
		ServiceID:     1,
		ServiceName:   "Corte",
		Price:         50,
		Duration:      30,
		PersonnelID:   nil, // pre-attribution row
	}
	require.NoError(t, db.Create(legacy).Error)

	stats := BackfillReservationServicePersonnel()
	assert.Equal(t, 1, stats.Eligible, "one NULL line is eligible")
	assert.Equal(t, 1, stats.Filled, "the NULL line is filled from the lead")
	assert.Equal(t, 0, stats.Skipped)

	var stored models.ReservationService
	require.NoError(t, db.First(&stored, legacy.ID).Error)
	require.NotNil(t, stored.PersonnelID, "line must no longer be NULL")
	assert.Equal(t, uint(7), *stored.PersonnelID, "line carries the reservation's lead")

	// Second run: no NULL rows remain, so it is a no-op.
	second := BackfillReservationServicePersonnel()
	assert.Equal(t, 0, second.Eligible)
	assert.Equal(t, 0, second.Filled)
}

// TestBackfillReservationServicePersonnel_SkipsOrphan proves a line whose
// reservation cannot be resolved is logged and skipped, never failing the run.
func TestBackfillReservationServicePersonnel_SkipsOrphan(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := newSQLiteDB(t)
	migrateReservationPersonnelSchema(t, db)
	DB = db

	orphan := &models.ReservationService{
		ReservationID: 999, // no such reservation
		ServiceID:     1,
		ServiceName:   "Corte",
		Price:         50,
		Duration:      30,
		PersonnelID:   nil,
	}
	require.NoError(t, db.Create(orphan).Error)

	assert.NotPanics(t, func() {
		stats := BackfillReservationServicePersonnel()
		assert.Equal(t, 1, stats.Skipped)
		assert.Equal(t, 0, stats.Filled)
	})

	var stored models.ReservationService
	require.NoError(t, db.First(&stored, orphan.ID).Error)
	assert.Nil(t, stored.PersonnelID, "orphan line is left untouched")
}

// TestBackfillReservationServicePersonnel_DoesNotOverwrite pins the
// non-destructive guarantee: a line that already carries a professional is
// never rewritten, even when it differs from the reservation's lead.
func TestBackfillReservationServicePersonnel_DoesNotOverwrite(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := newSQLiteDB(t)
	migrateReservationPersonnelSchema(t, db)
	DB = db

	res := &models.Reservation{
		BeautyCenterID: 1,
		ClientID:       1,
		PersonnelID:    7,
		Date:           time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
		StartTime:      "10:00",
		EndTime:        "11:00",
		TotalPrice:     100,
		Status:         "PENDING",
	}
	require.NoError(t, db.Create(res).Error)

	existing := uint(9)
	line := &models.ReservationService{
		ReservationID: res.ID,
		ServiceID:     1,
		ServiceName:   "Corte",
		Price:         50,
		Duration:      30,
		PersonnelID:   &existing,
	}
	require.NoError(t, db.Create(line).Error)

	stats := BackfillReservationServicePersonnel()
	assert.Equal(t, 0, stats.Eligible, "non-NULL lines are not eligible")
	assert.Equal(t, 0, stats.Filled)

	var stored models.ReservationService
	require.NoError(t, db.First(&stored, line.ID).Error)
	require.NotNil(t, stored.PersonnelID)
	assert.Equal(t, uint(9), *stored.PersonnelID, "existing attribution is preserved")
}
