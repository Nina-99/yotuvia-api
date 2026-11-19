package service

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupDepositTestDB wires the deposit flow with a center that requires seña
// (30% below trust 70), one service (100 BOB), staff and two clients.
func setupDepositTestDB(t *testing.T) (*gorm.DB, *ReservationServiceImpl, uint, uint, uint, uint) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.BeautyCenter{},
		&models.Client{},
		&models.Personnel{},
		&models.Category{},
		&models.Service{},
		&models.Reservation{},
		&models.ReservationService{},
		&models.Review{}, // FindByID aggregates ratings from reviews
		&models.BeautyCenterImage{},
		&models.WorkingHour{},
		&models.SocialMedia{},
		&models.Notification{},
		&models.PushDevice{},
	))

	notifSvc := NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)

	center := models.BeautyCenter{
		Name: "Sena Center", Email: "sena@test.com", Phone: "123", Password: "x", IsActive: true,
		DepositRequired: true, DepositPercent: 30, DepositTrustThreshold: 70, DepositHoldMinutes: 30,
	}
	require.NoError(t, db.Create(&center).Error)
	// The client create path now enforces posted hours; publish a full week so
	// the deposit scenarios keep booking real slots.
	seedFullWeekWorkingHours(t, db, center.ID)
	staff := models.Personnel{BeautyCenterID: center.ID, FirstName: "Ana", Phone: "456", IsActive: true}
	require.NoError(t, db.Create(&staff).Error)
	cat := models.Category{BeautyCenterID: center.ID, Name: "Peluquería"}
	require.NoError(t, db.Create(&cat).Error)
	svc := models.Service{BeautyCenterID: center.ID, CategoryID: cat.ID, Name: "Corte", Description: "Corte", Duration: 30, Price: 100, IsActive: true}
	require.NoError(t, db.Create(&svc).Error)
	trusted := models.Client{FirstName: "Buena", LastName: "Paga", Username: "buena_p", Phone: "111", Email: "buena@test.com", Password: "x", IsActive: true}
	require.NoError(t, db.Create(&trusted).Error)
	risky := models.Client{FirstName: "Mala", LastName: "Paga", Username: "mala_p", Phone: "222", Email: "mala@test.com", Password: "x", IsActive: true}
	require.NoError(t, db.Create(&risky).Error)
	require.NoError(t, db.Model(&models.Client{}).Where("id = ?", risky.ID).Update("trust_score", 50).Error)

	s := &ReservationServiceImpl{
		ReservationRepository: repository.NewReservationRepositoryImpl(db),
		ServiceRepository:     repository.NewServiceRepositoryImpl(db),
		PersonnelRepository:   repository.NewPersonnelRepositoryImpl(db),
		ClientRepository:      repository.NewClientRepositoryImpl(db),
		CenterRepository:      repository.NewBeautyCenterRepositoryImpl(db),
		NotificationService:   notifSvc,
		Validate:              validator.New(),
	}
	return db, s, center.ID, trusted.ID, risky.ID, svc.ID
}

// seedFullWeekWorkingHours publishes an 08:00–20:00 block for every weekday so
// the client create path (which now enforces posted hours) keeps accepting the
// bookings these service-level scenarios make.
func seedFullWeekWorkingHours(t *testing.T, db *gorm.DB, centerID uint) {
	t.Helper()
	for day := 0; day < 7; day++ {
		require.NoError(t, db.Create(&models.WorkingHour{
			BeautyCenterID: centerID,
			DayOfWeek:      day,
			StartTime:      "08:00",
			EndTime:        "20:00",
		}).Error)
	}
}

// Proof references the API itself issues, shaped like the URL returned by
// POST /api/upload/receipt. Anything else is rejected by UploadProof.
const (
	validProofPath1 = "/uploads/receipts/11111111-1111-1111-1111-111111111111.png"
	validProofPath2 = "/uploads/receipts/22222222-2222-2222-2222-222222222222.jpg"
)

func depositCreateReq(centerID, clientID, staffID, serviceID uint, start string) request.CreateReservationRequest {
	return request.CreateReservationRequest{
		BeautyCenterID: centerID,
		ClientID:       clientID,
		PersonnelID:    staffID,
		ServiceIDs:     []uint{serviceID},
		Date:           time.Now().AddDate(0, 0, 3).Format("2006-01-02"),
		StartTime:      start,
	}
}

func findRes(t *testing.T, db *gorm.DB, clientID uint) models.Reservation {
	t.Helper()
	var r models.Reservation
	require.NoError(t, db.Where("client_id = ?", clientID).First(&r).Error)
	return r
}

func TestComputeDepositQuote(t *testing.T) {
	tests := []struct {
		name           string
		required       bool
		fixed, percent float64
		total          float64
		want           float64
	}{
		{"disabled center means no deposit", false, 50, 30, 200, 0},
		{"fixed amount wins over percent", true, 50, 30, 200, 50},
		{"fixed capped at total", true, 500, 0, 200, 200},
		{"percent of total", true, 0, 30, 200, 60},
		{"percent capped at total", true, 0, 150, 200, 200},
		{"no amount configured", true, 0, 0, 200, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ComputeDepositQuote(tt.required, tt.fixed, tt.percent, tt.total))
		})
	}
}

func TestDepositFlow_CreateDecidesHold(t *testing.T) {
	db, s, centerID, trustedID, riskyID, serviceID := setupDepositTestDB(t)
	var staff models.Personnel
	require.NoError(t, db.First(&staff).Error)

	t.Run("trusted client skips deposit", func(t *testing.T) {
		require.NoError(t, s.Create(depositCreateReq(centerID, trustedID, staff.ID, serviceID, "10:00")))
		r := findRes(t, db, trustedID)
		assert.Equal(t, PaymentNone, r.PaymentStatus)
		assert.Equal(t, 0.0, r.DepositAmount)
		assert.Nil(t, r.HoldExpiresAt)
		assert.Equal(t, "PENDING", r.Status)
	})

	t.Run("distrusted client gets pending proof hold", func(t *testing.T) {
		require.NoError(t, s.Create(depositCreateReq(centerID, riskyID, staff.ID, serviceID, "11:00")))
		r := findRes(t, db, riskyID)
		assert.Equal(t, PaymentPendingProof, r.PaymentStatus)
		assert.Equal(t, 30.0, r.DepositAmount)
		require.NotNil(t, r.HoldExpiresAt)
		assert.True(t, r.HoldExpiresAt.After(time.Now()))
	})
}

func TestDepositFlow_UploadAndReview(t *testing.T) {
	db, s, centerID, _, riskyID, serviceID := setupDepositTestDB(t)
	var staff models.Personnel
	require.NoError(t, db.First(&staff).Error)
	require.NoError(t, s.Create(depositCreateReq(centerID, riskyID, staff.ID, serviceID, "11:00")))
	r := findRes(t, db, riskyID)

	t.Run("empty url is rejected", func(t *testing.T) {
		assert.ErrorIs(t, s.UploadProof(r.ID, riskyID, ""), apperrors.ErrBadRequest)
	})

	t.Run("other client is forbidden", func(t *testing.T) {
		assert.ErrorIs(t, s.UploadProof(r.ID, riskyID+999, validProofPath1), apperrors.ErrForbidden)
	})

	t.Run("upload stores proof and keeps hold", func(t *testing.T) {
		require.NoError(t, s.UploadProof(r.ID, riskyID, validProofPath1))
		var got models.Reservation
		require.NoError(t, db.First(&got, r.ID).Error)
		require.NotNil(t, got.ProofImageURL)
		assert.Equal(t, validProofPath1, *got.ProofImageURL)
		assert.Equal(t, PaymentPendingProof, got.PaymentStatus)
	})

	t.Run("other center cannot review", func(t *testing.T) {
		assert.ErrorIs(t, s.ReviewProof(r.ID, centerID+999, true), apperrors.ErrForbidden)
	})

	t.Run("reject allows re-upload then approve confirms", func(t *testing.T) {
		require.NoError(t, s.ReviewProof(r.ID, centerID, false))
		var got models.Reservation
		require.NoError(t, db.First(&got, r.ID).Error)
		assert.Equal(t, PaymentRejected, got.PaymentStatus)
		assert.Equal(t, "PENDING", got.Status)

		require.NoError(t, s.UploadProof(r.ID, riskyID, validProofPath2))
		require.NoError(t, s.ReviewProof(r.ID, centerID, true))
		require.NoError(t, db.First(&got, r.ID).Error)
		assert.Equal(t, PaymentApproved, got.PaymentStatus)
		assert.Equal(t, "CONFIRMED", got.Status)
	})

	t.Run("double review is rejected", func(t *testing.T) {
		assert.ErrorIs(t, s.ReviewProof(r.ID, centerID, true), apperrors.ErrConflict)
	})
}

func TestDepositFlow_Expiry(t *testing.T) {
	db, s, centerID, _, riskyID, serviceID := setupDepositTestDB(t)
	var staff models.Personnel
	require.NoError(t, db.First(&staff).Error)
	require.NoError(t, s.Create(depositCreateReq(centerID, riskyID, staff.ID, serviceID, "11:00")))
	r := findRes(t, db, riskyID)

	// Force the hold into the past.
	past := time.Now().Add(-time.Hour)
	require.NoError(t, db.Model(&models.Reservation{}).Where("id = ?", r.ID).Update("hold_expires_at", past).Error)

	t.Run("upload on expired hold cancels it", func(t *testing.T) {
		err := s.UploadProof(r.ID, riskyID, validProofPath1)
		assert.ErrorIs(t, err, apperrors.ErrConflict)
		var got models.Reservation
		require.NoError(t, db.First(&got, r.ID).Error)
		assert.Equal(t, "CANCELLED", got.Status)
		assert.Equal(t, 50, trustOf(t, db, riskyID)) // no penalty: never confirmed
	})

	t.Run("sweep collects expired holds without penalty", func(t *testing.T) {
		other := models.Client{FirstName: "Otra", LastName: "X", Username: "otra_x", Phone: "333", Email: "otra@test.com", Password: "x", IsActive: true}
		require.NoError(t, db.Create(&other).Error)
		require.NoError(t, db.Model(&models.Client{}).Where("id = ?", other.ID).Update("trust_score", 10).Error)
		require.NoError(t, s.Create(depositCreateReq(centerID, other.ID, staff.ID, serviceID, "12:00")))
		var hold models.Reservation
		require.NoError(t, db.Where("client_id = ?", other.ID).First(&hold).Error)
		require.NoError(t, db.Model(&models.Reservation{}).Where("id = ?", hold.ID).Update("hold_expires_at", past).Error)

		swept, err := s.ExpireHolds(time.Now())
		require.NoError(t, err)
		assert.Equal(t, 1, swept)
		assert.Equal(t, "CANCELLED", statusOf(t, db, hold.ID))
		assert.Equal(t, 10, trustOf(t, db, other.ID))
	})
}
