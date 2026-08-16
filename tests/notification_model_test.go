package tests

import (
	"encoding/json"
	"testing"

	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/stretchr/testify/assert"
)

// TestNotificationPersonnelIDRoundTrip — a staff notification row must persist
// its nullable PersonnelID and read it back unchanged (design D5: staff rows
// use PersonnelID set + ClientID sentinel 0).
func TestNotificationPersonnelIDRoundTrip(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewNotificationRepositoryImpl(db)

	personnelID := uint(7)
	notif := &models.Notification{
		ClientID:    0,
		PersonnelID: &personnelID,
		Type:        "reservation_cancelled",
		Title:       "Turno cancelado",
		Message:     "El turno fue cancelado",
	}
	assert.NoError(t, repo.Create(notif))

	loaded, err := repo.FindByID(notif.ID)
	assert.NoError(t, err)
	assert.Equal(t, uint(7), *loaded.PersonnelID)
	assert.Equal(t, uint(0), loaded.ClientID)
}

// TestNotificationClientRowKeepsPersonnelIDNil — a plain client notification
// (no PersonnelID set) must round-trip with a nil PersonnelID (D5: client rows
// unaffected).
func TestNotificationClientRowKeepsPersonnelIDNil(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewNotificationRepositoryImpl(db)

	notif := &models.Notification{
		ClientID: 5,
		Type:     "appointment_reminder",
		Title:    "Recordatorio",
		Message:  "Tu turno es mañana",
	}
	assert.NoError(t, repo.Create(notif))

	loaded, err := repo.FindByID(notif.ID)
	assert.NoError(t, err)
	assert.Nil(t, loaded.PersonnelID)
	assert.Equal(t, uint(5), loaded.ClientID)
}

// TestNotificationResponsePersonnelID — the NotificationResponse DTO must
// serialize personnel_id when a staff row is present and omit it for client
// rows (nil pointer + omitempty keeps the client wire contract unchanged).
func TestNotificationResponsePersonnelID(t *testing.T) {
	personnelID := uint(7)

	staffJSON, err := json.Marshal(response.NotificationResponse{
		ID:          1,
		ClientID:    0,
		PersonnelID: &personnelID,
		IsRead:      false,
	})
	assert.NoError(t, err)
	staffStr := string(staffJSON)
	assert.Contains(t, staffStr, `"personnel_id":7`, "staff row must carry personnel_id")

	clientJSON, err := json.Marshal(response.NotificationResponse{
		ID:       2,
		ClientID: 5,
		IsRead:   false,
	})
	assert.NoError(t, err)
	assert.NotContains(t, string(clientJSON), "personnel_id", "client row must omit personnel_id")
}

// TestVerifyQRResponseShape — the VerifyQR success DTO must expose both the
// reservation DTO and the already_completed flag (delta spec: first scan
// false, re-scan true).
func TestVerifyQRResponseShape(t *testing.T) {
	first, err := json.Marshal(response.VerifyQRResponse{
		Reservation:      response.ReservationResponse{ID: 42, Status: "COMPLETED"},
		AlreadyCompleted: false,
	})
	assert.NoError(t, err)
	assert.Contains(t, string(first), `"already_completed":false`)
	assert.Contains(t, string(first), `"reservation"`)
	assert.Contains(t, string(first), `"id":42`)

	rescan, err := json.Marshal(response.VerifyQRResponse{
		Reservation:      response.ReservationResponse{ID: 42, Status: "COMPLETED"},
		AlreadyCompleted: true,
	})
	assert.NoError(t, err)
	assert.Contains(t, string(rescan), `"already_completed":true`)
}
