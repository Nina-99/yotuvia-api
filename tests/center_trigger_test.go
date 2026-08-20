package tests

import (
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestReviewCenterNotifications_Created — a successful client review emits a
// center-scoped review_created row targeted at the review's BeautyCenterID,
// carrying review_id + rating (spec notification-api: "Review emits a center
// notification").
func TestReviewCenterNotifications_Created(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedClient(t, db, 5, "client5", "c5@example.com")
	// The review gate requires a completed reservation at that center.
	assert.NoError(t, db.Create(&models.Reservation{
		BeautyCenterID: 1, ClientID: 5, Status: "COMPLETED", Date: mustParseReviewDate(t), StartTime: "10:00", EndTime: "10:30", TotalPrice: 100,
	}).Error)

	notifSvc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)
	svc := service.NewReviewServiceImpl(repository.NewReviewRepositoryImpl(db), notifSvc, validator.New())

	assert.NoError(t, svc.Create(5, request.CreateReviewRequest{BeautyCenterID: 1, Rating: 5, Comment: "Great!"}))

	var n models.Notification
	assert.NoError(t, db.Where("type = ?", "review_created").First(&n).Error)
	assert.Equal(t, uint(0), n.ClientID, "D1 sentinel: center rows use ClientID=0")
	assert.Equal(t, uint(1), *n.BeautyCenterID, "targets the review's center")
	assert.Nil(t, n.PersonnelID)
	assert.Contains(t, n.Data, `"review_id":`, "Data carries the review id")
	assert.Contains(t, n.Data, `"rating":5`, "Data carries the rating")
}

// TestReviewCenterNotifications_DuplicateEmitsNothing — a rejected (duplicate)
// review returns an error and MUST NOT emit a second notification (spec:
// "Failed review emits nothing").
func TestReviewCenterNotifications_DuplicateEmitsNothing(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedClient(t, db, 5, "client5", "c5@example.com")
	assert.NoError(t, db.Create(&models.Reservation{
		BeautyCenterID: 1, ClientID: 5, Status: "COMPLETED", Date: mustParseReviewDate(t), StartTime: "10:00", EndTime: "10:30", TotalPrice: 100,
	}).Error)

	notifSvc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)
	svc := service.NewReviewServiceImpl(repository.NewReviewRepositoryImpl(db), notifSvc, validator.New())

	assert.NoError(t, svc.Create(5, request.CreateReviewRequest{BeautyCenterID: 1, Rating: 5, Comment: "Great!"}))
	err := svc.Create(5, request.CreateReviewRequest{BeautyCenterID: 1, Rating: 4, Comment: "Second"})
	assert.Error(t, err, "duplicate review must fail")

	var count int64
	assert.NoError(t, db.Model(&models.Notification{}).Where("type = ?", "review_created").Count(&count).Error)
	assert.Equal(t, int64(1), count, "no notification for the failed review")
}

// TestReservationCenterNotifications_Walkin — a center admin walk-in emits a
// center-scoped walkin_created row targeted at the actor's center, carrying
// reservation_id + client_id (spec reservation-api: "Center admin walk-in
// notifies the center").
func TestReservationCenterNotifications_Walkin(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedClient(t, db, 5, "client5", "c5@example.com")
	assert.NoError(t, db.Create(&models.Service{
		BeautyCenterID: 1, CategoryID: 1, Name: "Corte", Description: "corte", Duration: 30, Price: 100, IsActive: true,
	}).Error)

	svc := newReservationTriggerTestService(t, db)
	res, err := svc.CreateAdminReservation(request.CreateAdminReservationRequest{
		BeautyCenterID: 1, ClientID: 5, ServiceIDs: []uint{1}, Date: "2026-08-20", StartTime: "11:00",
	}, service.ReservationActor{ClientID: 1, CenterID: 1, Role: "beauty_center"})
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var n models.Notification
	assert.NoError(t, db.Where("type = ?", "walkin_created").First(&n).Error)
	assert.Equal(t, uint(0), n.ClientID, "D1 sentinel: center rows use ClientID=0")
	assert.Equal(t, uint(1), *n.BeautyCenterID, "targets the actor's center")
	assert.Nil(t, n.PersonnelID)
	assert.Equal(t, "Nueva reserva walk-in", n.Title)
	assert.Contains(t, n.Data, `"reservation_id":`, "Data carries the reservation id")
	assert.Contains(t, n.Data, `"client_id":5`, "Data carries the client id")
}

// TestReservationCenterNotifications_WalkinPersonnel — a personnel walk-in
// targets their membership center (the service already proves
// actor.CenterID == req.BeautyCenterID for personnel) (spec reservation-api:
// "Personnel walk-in notifies their center").
func TestReservationCenterNotifications_WalkinPersonnel(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedClient(t, db, 5, "client5", "c5@example.com")
	seedPersonnel(t, db, 7, 1)
	assert.NoError(t, db.Create(&models.Service{
		BeautyCenterID: 1, CategoryID: 1, Name: "Corte", Description: "corte", Duration: 30, Price: 100, IsActive: true,
	}).Error)

	svc := newReservationTriggerTestService(t, db)
	res, err := svc.CreateAdminReservation(request.CreateAdminReservationRequest{
		BeautyCenterID: 1, ClientID: 5, ServiceIDs: []uint{1}, Date: "2026-08-20", StartTime: "11:00",
	}, service.ReservationActor{ClientID: 7, CenterID: 1, Role: "personnel"})
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var n models.Notification
	assert.NoError(t, db.Where("type = ?", "walkin_created").First(&n).Error)
	assert.Equal(t, uint(1), *n.BeautyCenterID, "personnel walk-in targets the membership center")
	assert.Contains(t, n.Data, `"client_id":5`)
}

// mustParseReviewDate returns the fixed test date as time.Time.
func mustParseReviewDate(t *testing.T) (d time.Time) {
	t.Helper()
	d, err := time.Parse("2006-01-02", "2026-08-20")
	if err != nil {
		t.Fatalf("bad test date: %v", err)
	}
	return d
}
