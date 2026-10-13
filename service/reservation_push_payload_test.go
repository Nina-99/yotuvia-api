package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/stretchr/testify/assert"
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

// pushEnvelope decodes the {type, data} push payload; data must be a JSON
// string that carries the original reservation payload (design D3 / D6).
type pushEnvelope struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// TestAssignPersonnelPushUsesTypedEnvelope: the push payload sent by
// AssignPersonnel must be wrapped as {"type":"personnel_changed","data":<json string>}
// so the mobile mapper resolves the cold-start push to the reservations tab.
func TestAssignPersonnelPushUsesTypedEnvelope(t *testing.T) {
	staff := &models.Personnel{
		FirstName:      "Jane",
		LastName:       "Doe",
		BeautyCenterID: 10,
	}
	svc, notif := newAssignPersonnelTestService(pendingReservation(10, 5), staff)

	err := svc.AssignPersonnel(42, 7, 10)
	assert.NoError(t, err)
	assert.Len(t, notif.pushedData, 1, "one push must be sent")

	var envelope pushEnvelope
	assert.NoError(t, json.Unmarshal([]byte(notif.pushedData[0]), &envelope))
	assert.Equal(t, "personnel_changed", envelope.Type)

	// The inner data is a JSON string that round-trips to the original payload.
	var inner map[string]uint
	assert.NoError(t, json.Unmarshal([]byte(envelope.Data), &inner))
	assert.Equal(t, uint(42), inner["reservation_id"])
	assert.Equal(t, uint(7), inner["personnel_id"])
}

// TestAssignPersonnelPushEnvelopeUsesCallArgs triangulates with different IDs
// to prove the envelope is built from the call arguments (not hardcoded) and
// that the row data stored via Create stays clean (no envelope).
func TestAssignPersonnelPushEnvelopeUsesCallArgs(t *testing.T) {
	staff := &models.Personnel{
		FirstName:      "Jane",
		LastName:       "Doe",
		BeautyCenterID: 10,
	}
	svc, notif := newAssignPersonnelTestService(pendingReservation(10, 5), staff)

	err := svc.AssignPersonnel(1, 2, 10)
	assert.NoError(t, err)
	assert.Len(t, notif.pushedData, 1)

	var envelope pushEnvelope
	assert.NoError(t, json.Unmarshal([]byte(notif.pushedData[0]), &envelope))
	var inner map[string]uint
	assert.NoError(t, json.Unmarshal([]byte(envelope.Data), &inner))
	assert.Equal(t, uint(1), inner["reservation_id"])
	assert.Equal(t, uint(2), inner["personnel_id"])

	// Row data (NotificationService.Create) must remain the raw payload,
	// untouched by the push envelope.
	assert.Len(t, notif.created, 1)
	var rowData map[string]uint
	assert.NoError(t, json.Unmarshal([]byte(notif.created[0]), &rowData))
	assert.Equal(t, uint(1), rowData["reservation_id"])
	assert.Equal(t, uint(2), rowData["personnel_id"])
}
