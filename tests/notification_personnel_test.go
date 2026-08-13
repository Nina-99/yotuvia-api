package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ptrUint returns a pointer to the given uint (for Notification.PersonnelID).
func ptrUint(v uint) *uint { return &v }

// seedPersonnelNotification inserts a staff notification row: ClientID=0
// sentinel + PersonnelID set (design D5). isRead controls the flag.
func seedPersonnelNotification(t *testing.T, db *gorm.DB, personnelID uint, notifType, title string, isRead bool) *models.Notification {
	t.Helper()
	n := &models.Notification{
		ClientID:    0,
		PersonnelID: ptrUint(personnelID),
		Type:        notifType,
		Title:       title,
		Message:     "message",
		IsRead:      isRead,
	}
	if err := db.Create(n).Error; err != nil {
		t.Fatalf("failed to seed personnel notification: %v", err)
	}
	return n
}

// TestNotificationRepo_FindByPersonnelID — staff notifications are scoped by
// PersonnelID; client rows (PersonnelID nil) never leak into staff results.
func TestNotificationRepo_FindByPersonnelID(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewNotificationRepositoryImpl(db)

	seedPersonnelNotification(t, db, 7, "reservation_cancelled", "for-staff-7", false)
	seedPersonnelNotification(t, db, 7, "personnel_changed", "for-staff-7-b", false)
	seedPersonnelNotification(t, db, 9, "reservation_cancelled", "for-staff-9", false)
	// Client row — must never appear in staff queries.
	db.Create(&models.Notification{ClientID: 5, Type: "appointment_reminder", Title: "client-row", Message: "m"})

	rows, err := repo.FindByPersonnelID(7)
	assert.NoError(t, err)
	assert.Len(t, rows, 2, "only staff-7 rows")
	for _, r := range rows {
		assert.Equal(t, uint(7), *r.PersonnelID)
		assert.Equal(t, uint(0), r.ClientID, "staff rows carry the ClientID=0 sentinel")
		assert.NotEqual(t, "client-row", r.Title)
	}
}

// TestNotificationRepo_FindUnreadByPersonnelID — unread filter applies to the
// staff scope only.
func TestNotificationRepo_FindUnreadByPersonnelID(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewNotificationRepositoryImpl(db)

	seedPersonnelNotification(t, db, 7, "reservation_cancelled", "unread-1", false)
	seedPersonnelNotification(t, db, 7, "personnel_changed", "read-1", true)
	seedPersonnelNotification(t, db, 7, "reservation_cancelled", "unread-2", false)
	seedPersonnelNotification(t, db, 9, "reservation_cancelled", "other-unread", false)

	rows, err := repo.FindUnreadByPersonnelID(7)
	assert.NoError(t, err)
	assert.Len(t, rows, 2, "staff-7 unread only")
	assert.Equal(t, "unread-2", rows[0].Title, "newest first (created_at DESC)")
	assert.Equal(t, "unread-1", rows[1].Title)
}

// TestNotificationRepo_CountUnreadByPersonnelID — count is per-staff and
// ignores read rows and other staff members' unread.
func TestNotificationRepo_CountUnreadByPersonnelID(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewNotificationRepositoryImpl(db)

	seedPersonnelNotification(t, db, 7, "reservation_cancelled", "unread", false)
	seedPersonnelNotification(t, db, 7, "personnel_changed", "read", true)
	seedPersonnelNotification(t, db, 7, "reservation_cancelled", "unread-2", false)
	seedPersonnelNotification(t, db, 9, "reservation_cancelled", "other-unread", false)

	count, err := repo.CountUnreadByPersonnelID(7)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)

	count9, err := repo.CountUnreadByPersonnelID(9)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count9, "staff 9 sees only its own unread")
}

// TestNotificationService_CreateForPersonnel — the sentinel row is persisted
// with ClientID=0 and PersonnelID set; reading it back via the client path must
// NOT return it (client_id = 0 never matches a real client).
func TestNotificationService_CreateForPersonnel(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewNotificationRepositoryImpl(db)
	svc := service.NewNotificationServiceImpl(repo, repository.NewPushDeviceRepositoryImpl(db))

	err := svc.CreateForPersonnel(7, "reservation_cancelled", "Turno cancelado", "Tu turno fue cancelado", `{"reservation_id":42}`)
	assert.NoError(t, err)

	var stored models.Notification
	assert.NoError(t, db.First(&stored, "title = ?", "Turno cancelado").Error)
	assert.Equal(t, uint(0), stored.ClientID, "D5 sentinel: staff rows use ClientID=0")
	assert.Equal(t, uint(7), *stored.PersonnelID)

	// Client finder must not surface staff rows.
	clientRows, err := repo.FindByClientID(7)
	assert.NoError(t, err)
	assert.Empty(t, clientRows, "staff row must not appear in client queries")
}

// TestNotificationService_PersonnelOwnership — personnel methods are
// claim-scoped: a different personnelID is rejected, mark-read requires the
// notification to belong to the actor, and a client row cannot be claimed.
func TestNotificationService_PersonnelOwnership(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewNotificationRepositoryImpl(db)
	svc := service.NewNotificationServiceImpl(repo, repository.NewPushDeviceRepositoryImpl(db))

	myRow := seedPersonnelNotification(t, db, 7, "reservation_cancelled", "mine", false)
	otherRow := seedPersonnelNotification(t, db, 9, "reservation_cancelled", "theirs", false)
	clientRow := db.Create(&models.Notification{ClientID: 5, Type: "appointment_reminder", Title: "client", Message: "m"})
	if clientRow.Error != nil {
		t.Fatalf("failed to seed client row: %v", clientRow.Error)
	}

	// Reading another staff member's notifications → 403.
	_, err := svc.GetByPersonnelID(9, 7)
	assert.ErrorIs(t, err, apperrors.ErrForbidden)

	// Own list returns only own rows.
	rows, err := svc.GetByPersonnelID(7, 7)
	assert.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "mine", rows[0].Title)

	// Unread + count scoping.
	unread, err := svc.GetUnreadByPersonnelID(7, 7)
	assert.NoError(t, err)
	assert.Len(t, unread, 1)
	count, err := svc.CountUnreadByPersonnelID(7, 7)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Cross-staff mark-read → 403 and the row stays unread.
	err = svc.MarkAsReadByPersonnel(otherRow.ID, 7)
	assert.ErrorIs(t, err, apperrors.ErrForbidden)

	// Claiming a client row as staff → 403 (nil-safe ownership).
	err = svc.MarkAsReadByPersonnel(123456789, 7)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)

	// Owner marks own row read → OK, flag flips.
	err = svc.MarkAsReadByPersonnel(myRow.ID, 7)
	assert.NoError(t, err)
	var after models.Notification
	assert.NoError(t, db.First(&after, myRow.ID).Error)
	assert.True(t, after.IsRead)
}

// TestNotificationPersonnel_Routes — HTTP-layer authz for the personnel
// notification endpoints: personnel-only, claim-scoped, cross-staff 403,
// client rows keep PersonnelID null, unread/count accurate.
func TestNotificationPersonnel_Routes(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := authzTestRouter(t)

	staff7 := seedPersonnel(t, db, 7, 1)
	staff9 := seedPersonnel(t, db, 9, 1)
	_ = staff7
	_ = staff9

	n1 := seedPersonnelNotification(t, db, 7, "reservation_cancelled", "staff-7-unread", false)
	seedPersonnelNotification(t, db, 7, "personnel_changed", "staff-7-read", true)
	seedPersonnelNotification(t, db, 9, "reservation_cancelled", "staff-9-row", false)

	t.Run("no token is 401", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/notifications/personnel", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("client role is 403", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/notifications/personnel", roleToken(t, 1, 0, "client"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("staff reads own notifications only", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/notifications/personnel", roleToken(t, 7, 1, "personnel"), "")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"title":"staff-7-unread"`)
		assert.Contains(t, w.Body.String(), `"title":"staff-7-read"`)
		assert.NotContains(t, w.Body.String(), `"title":"staff-9-row"`, "no cross-staff leak")
	})

	t.Run("unread endpoint filters", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/notifications/personnel/unread", roleToken(t, 7, 1, "personnel"), "")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"title":"staff-7-unread"`)
		assert.NotContains(t, w.Body.String(), `"title":"staff-7-read"`)
	})

	t.Run("count endpoint is accurate", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/notifications/personnel/count", roleToken(t, 7, 1, "personnel"), "")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"count":1`)
	})

	t.Run("cross-staff mark-read is 403", func(t *testing.T) {
		w := do(r, http.MethodPatch, fmt.Sprintf("/api/notifications/personnel/%d/read", n1.ID), roleToken(t, 9, 1, "personnel"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("owner marks own staff notification read", func(t *testing.T) {
		w := do(r, http.MethodPatch, fmt.Sprintf("/api/notifications/personnel/%d/read", n1.ID), roleToken(t, 7, 1, "personnel"), "")
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestNotificationPersonnel_ClientRowsUnaffected — client notification flows
// keep PersonnelID nil after staff rows are created (D5 regression guard).
func TestNotificationPersonnel_ClientRowsUnaffected(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := authzTestRouter(t)

	seedPersonnelNotification(t, db, 7, "reservation_cancelled", "staff-row", false)
	clientNotif := seedNotification(t, db, 5, "reservation", "client-row")

	var stored models.Notification
	assert.NoError(t, db.First(&stored, clientNotif.ID).Error)
	assert.Nil(t, stored.PersonnelID, "client rows must keep PersonnelID nil")
	assert.Equal(t, uint(5), stored.ClientID)

	// Client reading its own notifications still works and excludes staff rows.
	w := do(r, http.MethodGet, "/api/notifications/client/5", roleToken(t, 5, 0, "client"), "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"title":"client-row"`)
	assert.NotContains(t, w.Body.String(), `"title":"staff-row"`)
}
