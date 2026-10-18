package tests

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// newReservationTriggerTestService wires the real repositories (SQLite in
// memory) so the personnel-scoped triggers can be asserted on the DB.
func newReservationTriggerTestService(t *testing.T, db *gorm.DB) service.ReservationService {
	t.Helper()
	validate := validator.New()
	notifRepo := repository.NewNotificationRepositoryImpl(db)
	return service.NewReservationServiceImpl(
		repository.NewReservationRepositoryImpl(db),
		repository.NewServiceRepositoryImpl(db),
		repository.NewPersonnelRepositoryImpl(db),
		repository.NewClientRepositoryImpl(db),
		nil, // TokenAccountRepository not needed for these flows
		service.NewNotificationServiceImpl(notifRepo, repository.NewPushDeviceRepositoryImpl(db)),
		nil, // LoyaltyService stub not needed for cancel/assign
		nil, // PromotionService stub not needed for cancel/assign
		nil, // TokenAccountService stub not needed for cancel/assign
		nil, // TokenMissionService stub not needed for cancel/assign
		nil, // MarketConfigService not needed for cancel/assign
		validate,
		repository.NewBeautyCenterRepositoryImpl(db),
		nil, // FeedPostRepository: attribution not exercised here
		repository.NewPersonnelServiceRepositoryImpl(db),
		nil, // TimeBlockRepository: blocks not exercised here
	)
}

// TestReservationPersonnelNotifications_Assign — reassignment produces staff
// rows with the D5 sentinel (ClientID=0, PersonnelID set) for BOTH the losing
// and the gaining staff, and client rows stay PersonnelID-nil.
func TestReservationPersonnelNotifications_Assign(t *testing.T) {
	db := setupRouterAuthzDB(t)
	_ = seedClient(t, db, 5, "client5", "c5@example.com")
	seedPersonnel(t, db, 7, 1)
	seedPersonnel(t, db, 9, 1)

	res := &models.Reservation{
		BeautyCenterID: 1,
		ClientID:       5,
		PersonnelID:    7, // losing staff
		Date:           time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
		StartTime:      "11:00",
		EndTime:        "11:30",
		Status:         "PENDING",
		TotalPrice:     100,
	}
	assert.NoError(t, db.Create(res).Error)

	svc := newReservationTriggerTestService(t, db)
	assert.NoError(t, svc.AssignPersonnel(res.ID, 9, 1))

	var losing, gaining, clientRow models.Notification
	assert.NoError(t, db.Where("personnel_id = ?", 7).First(&losing).Error, "losing staff row")
	assert.NoError(t, db.Where("personnel_id = ?", 9).First(&gaining).Error, "gaining staff row")
	assert.NoError(t, db.Where("client_id = ?", 5).First(&clientRow).Error, "client row")

	// D5 sentinel + scope.
	assert.Equal(t, uint(0), losing.ClientID)
	assert.Equal(t, uint(7), *losing.PersonnelID)
	assert.Equal(t, uint(0), gaining.ClientID)
	assert.Equal(t, uint(9), *gaining.PersonnelID)
	assert.Nil(t, clientRow.PersonnelID, "client row must keep PersonnelID nil")

	// Types and payloads.
	assert.Equal(t, "personnel_changed", losing.Type)
	assert.Equal(t, "personnel_changed", gaining.Type)
	assert.Contains(t, losing.Data, `"reservation_id":`)
	assert.Contains(t, losing.Data, `"personnel_id":9`)
	assert.Contains(t, gaining.Data, `"personnel_id":9`)
}

// TestReservationPersonnelNotifications_Cancel — a client cancellation writes
// a reservation_cancelled staff row (sentinel + PersonnelID) and the client
// row is untouched.
func TestReservationPersonnelNotifications_Cancel(t *testing.T) {
	db := setupRouterAuthzDB(t)
	_ = seedClient(t, db, 5, "client5", "c5@example.com")
	seedPersonnel(t, db, 7, 1)

	res := &models.Reservation{
		BeautyCenterID: 1,
		ClientID:       5,
		PersonnelID:    7,
		Date:           time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
		StartTime:      "11:00",
		EndTime:        "11:30",
		Status:         "CONFIRMED",
		TotalPrice:     100,
	}
	assert.NoError(t, db.Create(res).Error)

	svc := newReservationTriggerTestService(t, db)
	assert.NoError(t, svc.Cancel(res.ID, 5, "imprevisto"))

	var staff models.Notification
	assert.NoError(t, db.Where("personnel_id = ?", 7).First(&staff).Error)
	assert.Equal(t, uint(0), staff.ClientID, "D5 sentinel")
	assert.Equal(t, uint(7), *staff.PersonnelID)
	assert.Equal(t, "reservation_cancelled", staff.Type)
	assert.Contains(t, staff.Message, "imprevisto", "reason is surfaced to staff")
	assert.Contains(t, staff.Data, `"reservation_id":`)
}

// TestReservationPersonnelNotifications_CancelWithoutStaff — reservations
// created without personnel (PersonnelID=0) must NOT panic or write staff rows.
func TestReservationPersonnelNotifications_CancelWithoutStaff(t *testing.T) {
	db := setupRouterAuthzDB(t)
	_ = seedClient(t, db, 5, "client5", "c5@example.com")

	res := &models.Reservation{
		BeautyCenterID: 1,
		ClientID:       5,
		PersonnelID:    0,
		Date:           time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
		StartTime:      "11:00",
		EndTime:        "11:30",
		Status:         "PENDING",
		TotalPrice:     100,
	}
	assert.NoError(t, db.Create(res).Error)

	svc := newReservationTriggerTestService(t, db)
	assert.NoError(t, svc.Cancel(res.ID, 5, ""))

	var staffCount int64
	assert.NoError(t, db.Model(&models.Notification{}).Where("personnel_id = ?", 0).Count(&staffCount).Error)
	assert.Zero(t, staffCount, "no staff notification for unstaffed reservation")
}
