package service

import (
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupPhase3TestDB wires reminders, sweeps and waitlist against sqlite with
// a plain center (no deposit, no cancel policy by default), staff, one
// service (60 min, 100 BOB) and two clients.
func setupPhase3TestDB(t *testing.T) (*gorm.DB, *ReservationServiceImpl, WaitlistService, uint, uint, uint, uint) {
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
		&models.Review{},
		&models.BeautyCenterImage{},
		&models.WorkingHour{},
		&models.SocialMedia{},
		&models.Notification{},
		&models.PushDevice{},
		&models.WaitlistEntry{},
	))

	center := models.BeautyCenter{Name: "Phase3 Center", Email: "p3@test.com", Phone: "123", Password: "x", IsActive: true}
	require.NoError(t, db.Create(&center).Error)
	staff := models.Personnel{BeautyCenterID: center.ID, FirstName: "Ana", Phone: "456", IsActive: true}
	require.NoError(t, db.Create(&staff).Error)
	cat := models.Category{BeautyCenterID: center.ID, Name: "Peluquería"}
	require.NoError(t, db.Create(&cat).Error)
	svc := models.Service{BeautyCenterID: center.ID, CategoryID: cat.ID, Name: "Corte", Description: "Corte", Duration: 60, Price: 100, IsActive: true}
	require.NoError(t, db.Create(&svc).Error)
	mkClient := func(user, email, phone string) uint {
		c := models.Client{FirstName: "Cli", LastName: "X", Username: user, Phone: phone, Email: email, Password: "x", IsActive: true}
		require.NoError(t, db.Create(&c).Error)
		return c.ID
	}
	alice := mkClient("alice_p3", "alice@test.com", "111")
	bob := mkClient("bob_p3", "bob@test.com", "222")

	notifSvc := NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)
	reservationRepo := repository.NewReservationRepositoryImpl(db)
	resSvc := &ReservationServiceImpl{
		ReservationRepository: reservationRepo,
		ServiceRepository:     repository.NewServiceRepositoryImpl(db),
		PersonnelRepository:   repository.NewPersonnelRepositoryImpl(db),
		ClientRepository:      repository.NewClientRepositoryImpl(db),
		CenterRepository:      repository.NewBeautyCenterRepositoryImpl(db),
		NotificationService:   notifSvc,
		Validate:              validator.New(),
	}
	waitSvc := NewWaitlistServiceImpl(
		repository.NewWaitlistRepositoryImpl(db),
		reservationRepo,
		resSvc,
		repository.NewServiceRepositoryImpl(db),
		repository.NewClientRepositoryImpl(db),
		notifSvc,
		validator.New(),
	)
	return db, resSvc, waitSvc, center.ID, alice, bob, svc.ID
}

func p3Reservation(db *gorm.DB, centerID, clientID, staffID uint, at time.Time, status string) uint {
	// Consistent with slotStart/slotEnd: UTC calendar + UTC wall time.
	utc := at.UTC()
	r := models.Reservation{
		BeautyCenterID: centerID,
		ClientID:       clientID,
		PersonnelID:    staffID,
		Date:           time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC),
		StartTime:      utc.Format("15:04"),
		EndTime:        utc.Add(time.Hour).Format("15:04"),
		TotalPrice:     100,
		Status:         status,
		PaymentStatus:  PaymentNone,
	}
	db.Create(&r)
	return r.ID
}

func TestSendDueReminders(t *testing.T) {
	db, svc, _, centerID, alice, _, _ := setupPhase3TestDB(t)
	var staff models.Personnel
	require.NoError(t, db.First(&staff).Error)
	now := time.Now()
	in23h := now.Add(23 * time.Hour)
	in1h := now.Add(90 * time.Minute)
	id24 := p3Reservation(db, centerID, alice, staff.ID, in23h, "CONFIRMED")
	id2 := p3Reservation(db, centerID, alice, staff.ID, in1h, "CONFIRMED")
	// Pending reservations never get reminders.
	p3Reservation(db, centerID, alice, staff.ID, in1h, "PENDING")

	sent, err := svc.SendDueReminders(now)
	require.NoError(t, err)
	assert.Equal(t, 2, sent)

	var r24, r2 models.Reservation
	require.NoError(t, db.First(&r24, id24).Error)
	require.NoError(t, db.First(&r2, id2).Error)
	assert.NotNil(t, r24.Reminder24HSentAt)
	assert.Nil(t, r24.Reminder2HSentAt)
	assert.NotNil(t, r2.Reminder2HSentAt)

	// Second run is a no-op: one-shot flags.
	sent, err = svc.SendDueReminders(now)
	require.NoError(t, err)
	assert.Equal(t, 0, sent)
}

func TestSweepNoShows(t *testing.T) {
	db, svc, _, centerID, alice, _, _ := setupPhase3TestDB(t)
	var staff models.Personnel
	require.NoError(t, db.First(&staff).Error)
	now := time.Now()
	pastID := p3Reservation(db, centerID, alice, staff.ID, now.Add(-2*time.Hour), "CONFIRMED")
	futureID := p3Reservation(db, centerID, alice, staff.ID, now.Add(2*time.Hour), "CONFIRMED")
	pendingID := p3Reservation(db, centerID, alice, staff.ID, now.Add(-2*time.Hour), "PENDING")

	struck, err := svc.SweepNoShows(now, 30)
	require.NoError(t, err)
	assert.Equal(t, 1, struck)
	assert.Equal(t, "NO_SHOW", statusOf(t, db, pastID))
	assert.Equal(t, 80, trustOf(t, db, alice))
	assert.Equal(t, "CONFIRMED", statusOf(t, db, futureID))
	assert.Equal(t, "PENDING", statusOf(t, db, pendingID))
}

func TestCancelLatePolicy(t *testing.T) {
	db, svc, _, centerID, alice, _, _ := setupPhase3TestDB(t)
	var staff models.Personnel
	require.NoError(t, db.First(&staff).Error)
	// Opt the center into a 4h late window.
	require.NoError(t, db.Model(&models.BeautyCenter{}).Where("id = ?", centerID).Update("cancel_deadline_hours", 4).Error)
	now := time.Now()

	t.Run("early cancel keeps trust", func(t *testing.T) {
		id := p3Reservation(db, centerID, alice, staff.ID, now.Add(24*time.Hour), "CONFIRMED")
		require.NoError(t, svc.Cancel(id, alice, ""))
		assert.Equal(t, "CANCELLED", statusOf(t, db, id))
		assert.Equal(t, 100, trustOf(t, db, alice))
	})

	t.Run("late cancel strikes half", func(t *testing.T) {
		id := p3Reservation(db, centerID, alice, staff.ID, now.Add(time.Hour), "CONFIRMED")
		require.NoError(t, svc.Cancel(id, alice, ""))
		assert.Equal(t, "CANCELLED", statusOf(t, db, id))
		assert.Equal(t, 90, trustOf(t, db, alice))
	})
}

func TestWaitlistFlow(t *testing.T) {
	db, _, waitSvc, centerID, alice, bob, serviceID := setupPhase3TestDB(t)
	var staff models.Personnel
	require.NoError(t, db.First(&staff).Error)
	slotDate := time.Now().AddDate(0, 0, 5).Format("2006-01-02")
	join := func(clientID uint) (*models.WaitlistEntry, error) {
		return waitSvc.Join(request.JoinWaitlistRequest{
			BeautyCenterID: centerID,
			Date:           slotDate,
			StartTime:      "10:00",
			PersonnelID:    staff.ID,
			ServiceIDs:     []uint{serviceID},
		}, clientID)
	}

	t.Run("join then duplicate is conflict", func(t *testing.T) {
		e, err := join(alice)
		require.NoError(t, err)
		assert.Equal(t, models.WaitlistWaiting, e.Status)
		assert.Equal(t, "10:00", e.StartTime)
		assert.Equal(t, "11:00", e.EndTime) // 60-min service bundle
		_, err = join(alice)
		assert.ErrorIs(t, err, apperrors.ErrConflict)
	})

	t.Run("past slot is rejected", func(t *testing.T) {
		_, err := waitSvc.Join(request.JoinWaitlistRequest{
			BeautyCenterID: centerID,
			Date:           time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
			StartTime:      "10:00",
			PersonnelID:    staff.ID,
			ServiceIDs:     []uint{serviceID},
		}, bob)
		assert.ErrorIs(t, err, apperrors.ErrBadRequest)
	})

	t.Run("fifo offer then claim books", func(t *testing.T) {
		_, err := join(bob)
		require.NoError(t, err)
		parsed, _ := time.Parse("2006-01-02", slotDate)
		first, err := waitSvc.OfferForFreedSlot(centerID, parsed, "10:00")
		require.NoError(t, err)
		require.NotNil(t, first)
		// Alice joined first.
		var aliceEntry models.WaitlistEntry
		require.NoError(t, db.Where("client_id = ?", alice).First(&aliceEntry).Error)
		assert.Equal(t, first.ID, aliceEntry.ID)
		assert.Equal(t, models.WaitlistOffered, aliceEntry.Status)

		// Bob cannot claim Alice's offer.
		_, err = waitSvc.Claim(first.ID, bob)
		assert.ErrorIs(t, err, apperrors.ErrForbidden)

		booked, err := waitSvc.Claim(first.ID, alice)
		require.NoError(t, err)
		assert.Equal(t, "PENDING", booked.Status) // no deposit configured
		var done models.WaitlistEntry
		require.NoError(t, db.First(&done, first.ID).Error)
		assert.Equal(t, models.WaitlistBooked, done.Status)
	})

	t.Run("expired offer passes to next waiter", func(t *testing.T) {
		slot2 := time.Now().AddDate(0, 0, 6).Format("2006-01-02")
		joinAt := func(clientID uint) {
			_, err := waitSvc.Join(request.JoinWaitlistRequest{
				BeautyCenterID: centerID, Date: slot2, StartTime: "14:00",
				PersonnelID: staff.ID, ServiceIDs: []uint{serviceID},
			}, clientID)
			require.NoError(t, err)
		}
		joinAt(alice)
		joinAt(bob)
		parsed, _ := time.Parse("2006-01-02", slot2)
		first, err := waitSvc.OfferForFreedSlot(centerID, parsed, "14:00")
		require.NoError(t, err)
		require.NotNil(t, first)
		// Backdate the window into the past.
		past := time.Now().Add(-time.Hour)
		require.NoError(t, db.Model(&models.WaitlistEntry{}).Where("id = ?", first.ID).Update("offer_expires_at", past).Error)

		n, err := waitSvc.SweepExpiredOffers(time.Now())
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		// Bob may hold older entries from previous subtests: scope by slot.
		entries, err := waitSvc.ListMine(bob, false)
		require.NoError(t, err)
		var second *models.WaitlistEntry
		for i := range entries {
			if entries[i].StartTime == "14:00" {
				second = &entries[i]
				break
			}
		}
		require.NotNil(t, second)
		assert.Equal(t, models.WaitlistOffered, second.Status)
	})

	t.Run("leave cancels own entries", func(t *testing.T) {
		entries, err := waitSvc.ListMine(bob, false)
		require.NoError(t, err)
		require.NotEmpty(t, entries)
		for _, e := range entries {
			require.NoError(t, waitSvc.Leave(e.ID, bob))
		}
		after, err := waitSvc.ListMine(bob, false)
		require.NoError(t, err)
		assert.Empty(t, after)
	})
}
