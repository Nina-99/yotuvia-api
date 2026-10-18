package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeNotificationService captures Create/SendPushToClient payloads while
// satisfying the NotificationService interface (the real impl performs an
// external Expo Push API call).
type fakeNotificationService struct {
	NotificationService
	created      []string
	pushedData   []string
	createdStaff []string
}

func (f *fakeNotificationService) Create(clientID uint, notifType, title, message, data string) error {
	f.created = append(f.created, data)
	return nil
}

// CreateForPersonnel stubs the personnel-scoped trigger (design D5) so
// AssignPersonnel/Cancel don't panic through the nil embedded interface.
func (f *fakeNotificationService) CreateForPersonnel(personnelID uint, notifType, title, message, data string) error {
	f.createdStaff = append(f.createdStaff, data)
	return nil
}

func (f *fakeNotificationService) SendPushToClient(clientID uint, title, body, dataJSON string) error {
	f.pushedData = append(f.pushedData, dataJSON)
	return nil
}

// fakeReservationRepository stubs only the calls AssignPersonnel makes; any
// other method panics through the embedded nil interface (unused here).
type fakeReservationRepository struct {
	repository.ReservationRepository
	res *models.Reservation
}

func (f *fakeReservationRepository) FindById(reservationID uint) (*models.Reservation, error) {
	return f.res, nil
}

func (f *fakeReservationRepository) CheckOverlap(centerID uint, date time.Time, startTime, endTime string, personnelID, excludeID uint) (bool, error) {
	return false, nil
}

func (f *fakeReservationRepository) UpdatePersonnelID(reservationID uint, personnelID uint) error {
	return nil
}

// fakePersonnelRepository stubs only FindByID for the AssignPersonnel path.
type fakePersonnelRepository struct {
	repository.PersonnelRepository
	p *models.Personnel
}

func (f *fakePersonnelRepository) FindByID(id uint) (*models.Personnel, error) {
	return f.p, nil
}

func newAssignPersonnelTestService(res *models.Reservation, staff *models.Personnel) (*ReservationServiceImpl, *fakeNotificationService) {
	notif := &fakeNotificationService{}
	svc := &ReservationServiceImpl{
		ReservationRepository: &fakeReservationRepository{res: res},
		PersonnelRepository:   &fakePersonnelRepository{p: staff},
		NotificationService:   notif,
	}
	return svc, notif
}

func pendingReservation(beautyCenterID, clientID uint) *models.Reservation {
	return &models.Reservation{
		BeautyCenterID: beautyCenterID,
		ClientID:       clientID,
		Date:           time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		StartTime:      "10:00",
		EndTime:        "10:30",
		Status:         "PENDING",
	}
}

// TestAssignPersonnelClientNotificationCarriesType: the reassignment
// notification must carry the TYPE inside its own payload, because
// NotificationService.Create owns delivery now — this payload IS what the
// device receives, and routing depends on the type being present.
func TestAssignPersonnelClientNotificationCarriesType(t *testing.T) {
	staff := &models.Personnel{
		FirstName:      "Jane",
		LastName:       "Doe",
		BeautyCenterID: 10,
	}
	svc, notif := newAssignPersonnelTestService(pendingReservation(10, 5), staff)

	err := svc.AssignPersonnel(42, 7, 10)
	assert.NoError(t, err)
	require.Len(t, notif.created, 1)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(notif.created[0]), &payload))
	assert.Equal(t, "personnel_changed", payload["type"])
	assert.Equal(t, float64(42), payload["reservation_id"])
	assert.Equal(t, float64(7), payload["personnel_id"])

	// Create pushes on its own; a second explicit push would deliver the same
	// event twice.
	assert.Empty(t, notif.pushedData, "the push must not be sent twice")
}

// TestAssignPersonnelNotificationUsesCallArgs triangulates with different IDs to
// prove the payload is built from the call arguments, not hardcoded.
func TestAssignPersonnelNotificationUsesCallArgs(t *testing.T) {
	staff := &models.Personnel{
		FirstName:      "Jane",
		LastName:       "Doe",
		BeautyCenterID: 10,
	}
	svc, notif := newAssignPersonnelTestService(pendingReservation(10, 5), staff)

	err := svc.AssignPersonnel(1, 2, 10)
	assert.NoError(t, err)
	require.Len(t, notif.created, 1)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(notif.created[0]), &payload))
	assert.Equal(t, "personnel_changed", payload["type"])
	assert.Equal(t, float64(1), payload["reservation_id"])
	assert.Equal(t, float64(2), payload["personnel_id"])
}
