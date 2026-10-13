package tests

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
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

// TestNotificationRepo_ListCap — the client list is bounded by the repository's
// notificationListLimit so a long-lived account cannot pull an unbounded
// payload. It proves three things: (1) the list length equals the cap, (2) the
// cap keeps the NEWEST rows (created_at DESC), and (3) the unread COUNT still
// reports the true total — the badge is served by a separate COUNT(*) and must
// not desync from the list.
func TestNotificationRepo_ListCap(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewNotificationRepositoryImpl(db)

	// Mirrors repository.notificationListLimit (unexported). Keep in sync.
	const listLimit = 100
	const seeded = listLimit + 5

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < seeded; i++ {
		n := &models.Notification{
			ClientID: 5,
			Type:     "appointment_reminder",
			Title:    fmt.Sprintf("notif-%03d", i),
			Message:  "message",
			IsRead:   false,
		}
		// Distinct, increasing timestamps make "newest" deterministic — a tight
		// seed loop would otherwise collapse created_at to one value.
		n.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		if err := db.Create(n).Error; err != nil {
			t.Fatalf("failed to seed notification %d: %v", i, err)
		}
	}

	rows, err := repo.FindByClientID(5)
	assert.NoError(t, err)
	assert.Len(t, rows, listLimit, "list is capped at notificationListLimit")

	// Newest first: rows[0] is the last seeded row; only the five oldest are
	// dropped by the cap.
	assert.Equal(t, fmt.Sprintf("notif-%03d", seeded-1), rows[0].Title, "newest row survives the cap")
	assert.Equal(t, fmt.Sprintf("notif-%03d", seeded-listLimit), rows[len(rows)-1].Title, "oldest surviving row is exactly listLimit rows back")
	present := titlesOf(rows)
	for i := 0; i < seeded-listLimit; i++ {
		assert.NotContains(t, present, fmt.Sprintf("notif-%03d", i), "rows beyond the cap are dropped")
	}

	// The badge count is independent of the list cap: all unread rows count.
	count, err := repo.CountUnread(5)
	assert.NoError(t, err)
	assert.Equal(t, int64(seeded), count, "unread COUNT is unaffected by the list cap")
}

// titlesOf extracts the titles of a notification slice for membership checks.
func titlesOf(rows []models.Notification) []string {
	titles := make([]string, len(rows))
	for i, r := range rows {
		titles[i] = r.Title
	}
	return titles
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
