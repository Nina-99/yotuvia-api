package service

import (
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTrustTestDB wires the trust engine (reservation + client repos) against
// an in-memory DB with one center, one staff member and one client.
func setupTrustTestDB(t *testing.T) (*gorm.DB, *ReservationServiceImpl, uint, uint) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.BeautyCenter{},
		&models.Client{},
		&models.Personnel{},
		&models.Reservation{},
		&models.ReservationService{},
	))

	center := models.BeautyCenter{Name: "Trust Center", Email: "trust@test.com", Phone: "123", Password: "x", IsActive: true}
	require.NoError(t, db.Create(&center).Error)
	staff := models.Personnel{BeautyCenterID: center.ID, FirstName: "Ana", Phone: "456", IsActive: true}
	require.NoError(t, db.Create(&staff).Error)
	client := models.Client{FirstName: "Luis", LastName: "P", Username: "luis_trust", Phone: "789", Email: "luis@test.com", Password: "x", IsActive: true}
	require.NoError(t, db.Create(&client).Error)

	svc := &ReservationServiceImpl{
		ReservationRepository: repository.NewReservationRepositoryImpl(db),
		ClientRepository:      repository.NewClientRepositoryImpl(db),
	}
	return db, svc, center.ID, client.ID
}

func makeReservation(db *gorm.DB, centerID, clientID, staffID uint, date time.Time, status string) uint {
	r := models.Reservation{
		BeautyCenterID: centerID,
		ClientID:       clientID,
		PersonnelID:    staffID,
		Date:           date,
		StartTime:      "10:00",
		EndTime:        "10:30",
		TotalPrice:     100,
		Status:         status,
		PaymentStatus:  PaymentNone,
	}
	db.Create(&r)
	return r.ID
}

func trustOf(t *testing.T, db *gorm.DB, clientID uint) int {
	t.Helper()
	var c models.Client
	require.NoError(t, db.First(&c, clientID).Error)
	return c.TrustScore
}

func statusOf(t *testing.T, db *gorm.DB, reservationID uint) string {
	t.Helper()
	var r models.Reservation
	require.NoError(t, db.First(&r, reservationID).Error)
	return r.Status
}

func TestAdjustTrustScore_Clamp(t *testing.T) {
	tests := []struct {
		name  string
		start int
		delta int
		want  int
	}{
		{"no-show penalty from full score", 100, TrustNoShowPenalty, 80},
		{"penalty floors at zero", 10, TrustNoShowPenalty, 0},
		{"attendance reward caps at one hundred", 99, TrustAttendanceReward, 100},
		{"reward from zero", 0, TrustAttendanceReward, 2},
		{"zero delta keeps score", 50, 0, 50},
		{"repeated no-shows accumulate then floor", 30, -50, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, _, _, clientID := setupTrustTestDB(t)
			require.NoError(t, db.Model(&models.Client{}).Where("id = ?", clientID).Update("trust_score", tt.start).Error)
			got, err := repository.NewClientRepositoryImpl(db).AdjustTrustScore(clientID, tt.delta)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.want, trustOf(t, db, clientID))
		})
	}
}

func TestRequiresDeposit(t *testing.T) {
	tests := []struct {
		name      string
		score     int
		threshold int
		want      bool
	}{
		{"trusted client skips deposit", 100, DefaultDepositThreshold, false},
		{"threshold boundary is trusted", 70, 70, false},
		{"below threshold requires deposit", 69, 70, true},
		{"zero score requires deposit", 0, 70, true},
		{"custom strict threshold", 90, 95, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, svc, _, clientID := setupTrustTestDB(t)
			require.NoError(t, db.Model(&models.Client{}).Where("id = ?", clientID).Update("trust_score", tt.score).Error)
			got, err := svc.RequiresDeposit(clientID, tt.threshold)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("missing client is not found", func(t *testing.T) {
		_, svc, _, _ := setupTrustTestDB(t)
		_, err := svc.RequiresDeposit(9999, DefaultDepositThreshold)
		assert.ErrorIs(t, err, apperrors.ErrNotFound)
	})
}

func TestRecordNoShow(t *testing.T) {
	past := time.Now().AddDate(0, 0, -2)
	future := time.Now().AddDate(0, 0, 2)

	t.Run("past confirmed marks no-show and penalizes", func(t *testing.T) {
		db, svc, centerID, clientID := setupTrustTestDB(t)
		var staff models.Personnel
		require.NoError(t, db.First(&staff).Error)
		id := makeReservation(db, centerID, clientID, staff.ID, past, "CONFIRMED")

		require.NoError(t, svc.RecordNoShow(id, centerID))
		assert.Equal(t, "NO_SHOW", statusOf(t, db, id))
		assert.Equal(t, 80, trustOf(t, db, clientID))
	})

	t.Run("future slot is rejected without side effects", func(t *testing.T) {
		db, svc, centerID, clientID := setupTrustTestDB(t)
		var staff models.Personnel
		require.NoError(t, db.First(&staff).Error)
		id := makeReservation(db, centerID, clientID, staff.ID, future, "CONFIRMED")

		err := svc.RecordNoShow(id, centerID)
		assert.ErrorIs(t, err, apperrors.ErrConflict)
		assert.Equal(t, "CONFIRMED", statusOf(t, db, id))
		assert.Equal(t, 100, trustOf(t, db, clientID))
	})

	t.Run("pending reservation is rejected", func(t *testing.T) {
		db, svc, centerID, clientID := setupTrustTestDB(t)
		var staff models.Personnel
		require.NoError(t, db.First(&staff).Error)
		id := makeReservation(db, centerID, clientID, staff.ID, past, "PENDING")

		assert.ErrorIs(t, svc.RecordNoShow(id, centerID), apperrors.ErrConflict)
		assert.Equal(t, "PENDING", statusOf(t, db, id))
	})

	t.Run("already terminal no-show is rejected", func(t *testing.T) {
		db, svc, centerID, clientID := setupTrustTestDB(t)
		var staff models.Personnel
		require.NoError(t, db.First(&staff).Error)
		id := makeReservation(db, centerID, clientID, staff.ID, past, "NO_SHOW")

		assert.ErrorIs(t, svc.RecordNoShow(id, centerID), apperrors.ErrConflict)
		assert.Equal(t, 100, trustOf(t, db, clientID))
	})

	t.Run("other center is forbidden", func(t *testing.T) {
		db, svc, centerID, clientID := setupTrustTestDB(t)
		var staff models.Personnel
		require.NoError(t, db.First(&staff).Error)
		id := makeReservation(db, centerID, clientID, staff.ID, past, "CONFIRMED")

		assert.ErrorIs(t, svc.RecordNoShow(id, centerID+999), apperrors.ErrForbidden)
		assert.Equal(t, "CONFIRMED", statusOf(t, db, id))
	})

	t.Run("missing reservation is not found", func(t *testing.T) {
		_, svc, centerID, _ := setupTrustTestDB(t)
		assert.ErrorIs(t, svc.RecordNoShow(9999, centerID), apperrors.ErrNotFound)
	})
}

func TestValidPaymentStatus(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{PaymentNone, true},
		{PaymentPendingProof, true},
		{PaymentApproved, true},
		{PaymentRejected, true},
		{"", false},
		{"PAID", false},
		{"pending_proof", false},
	}
	for _, tt := range tests {
		t.Run("status "+tt.status, func(t *testing.T) {
			assert.Equal(t, tt.want, ValidPaymentStatus(tt.status))
		})
	}
}
