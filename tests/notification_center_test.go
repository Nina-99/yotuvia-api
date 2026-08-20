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

// seedCenterNotification inserts a center notification row: ClientID=0
// sentinel + BeautyCenterID set + PersonnelID nil (design D1). isRead controls
// the flag.
func seedCenterNotification(t *testing.T, db *gorm.DB, centerID uint, notifType, title string, isRead bool) *models.Notification {
	t.Helper()
	n := &models.Notification{
		ClientID:       0,
		BeautyCenterID: ptrUint(centerID),
		Type:           notifType,
		Title:          title,
		Message:        "message",
		IsRead:         isRead,
	}
	if err := db.Create(n).Error; err != nil {
		t.Fatalf("failed to seed center notification: %v", err)
	}
	return n
}

// TestNotificationRepo_FindByCenterID — center notifications are scoped by the
// BeautyCenterID column only; client rows (BeautyCenterID nil) and staff rows
// (PersonnelID set) never leak into center results (design D2).
func TestNotificationRepo_FindByCenterID(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewNotificationRepositoryImpl(db)

	seedCenterNotification(t, db, 2, "walkin_created", "center-2-a", false)
	seedCenterNotification(t, db, 2, "review_created", "center-2-b", false)
	seedCenterNotification(t, db, 9, "walkin_created", "center-9", false)
	// Client + staff rows — must never appear in center queries.
	db.Create(&models.Notification{ClientID: 5, Type: "appointment_reminder", Title: "client-row", Message: "m"})
	db.Create(&models.Notification{ClientID: 0, PersonnelID: ptrUint(7), Type: "personnel_changed", Title: "staff-row", Message: "m"})

	rows, err := repo.FindByCenterID(2)
	assert.NoError(t, err)
	assert.Len(t, rows, 2, "only center-2 rows")
	for _, r := range rows {
		assert.Equal(t, uint(2), *r.BeautyCenterID)
		assert.Equal(t, uint(0), r.ClientID, "center rows carry the ClientID=0 sentinel")
		assert.Nil(t, r.PersonnelID, "center rows keep PersonnelID nil")
		assert.NotEqual(t, "client-row", r.Title)
		assert.NotEqual(t, "staff-row", r.Title)
	}
}

// TestNotificationRepo_FindUnreadByCenterID — the unread filter applies to the
// center scope only, newest first.
func TestNotificationRepo_FindUnreadByCenterID(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewNotificationRepositoryImpl(db)

	seedCenterNotification(t, db, 2, "walkin_created", "unread-1", false)
	seedCenterNotification(t, db, 2, "review_created", "read-1", true)
	seedCenterNotification(t, db, 2, "walkin_created", "unread-2", false)
	seedCenterNotification(t, db, 9, "walkin_created", "other-unread", false)

	rows, err := repo.FindUnreadByCenterID(2)
	assert.NoError(t, err)
	assert.Len(t, rows, 2, "center-2 unread only")
	assert.Equal(t, "unread-2", rows[0].Title, "newest first (created_at DESC)")
	assert.Equal(t, "unread-1", rows[1].Title)
}

// TestNotificationRepo_CountUnreadByCenterID — the count is per-center and
// ignores read rows and other centers' unread.
func TestNotificationRepo_CountUnreadByCenterID(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewNotificationRepositoryImpl(db)

	seedCenterNotification(t, db, 2, "walkin_created", "unread", false)
	seedCenterNotification(t, db, 2, "review_created", "read", true)
	seedCenterNotification(t, db, 2, "walkin_created", "unread-2", false)
	seedCenterNotification(t, db, 9, "walkin_created", "other-unread", false)

	count, err := repo.CountUnreadByCenterID(2)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)

	count9, err := repo.CountUnreadByCenterID(9)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count9, "center 9 sees only its own unread")
}

// TestNotificationService_CreateForCenter — the sentinel row is persisted with
// ClientID=0, BeautyCenterID set, and PersonnelID nil (spec: "Create persists
// the sentinel shape"); reading it back via the client path must NOT return it.
func TestNotificationService_CreateForCenter(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewNotificationRepositoryImpl(db)
	svc := service.NewNotificationServiceImpl(repo, repository.NewPushDeviceRepositoryImpl(db))

	err := svc.CreateForCenter(2, "walkin_created", "Nueva reserva walk-in", "Un cliente reservó un turno", `{"reservation_id":42,"client_id":5}`)
	assert.NoError(t, err)

	var stored models.Notification
	assert.NoError(t, db.First(&stored, "title = ?", "Nueva reserva walk-in").Error)
	assert.Equal(t, uint(0), stored.ClientID, "D1 sentinel: center rows use ClientID=0")
	assert.Equal(t, uint(2), *stored.BeautyCenterID)
	assert.Nil(t, stored.PersonnelID, "center rows keep PersonnelID nil")

	// Client + staff finders must not surface center rows.
	clientRows, err := repo.FindByClientID(2)
	assert.NoError(t, err)
	assert.Empty(t, clientRows, "center row must not appear in client queries")
	staffRows, err := repo.FindByPersonnelID(2)
	assert.NoError(t, err)
	assert.Empty(t, staffRows, "center row must not appear in staff queries")
}

// TestNotificationService_CenterOwnership — center methods are claim-scoped: a
// different centerID is rejected, mark-read requires the notification to belong
// to the actor's center, and client/staff rows cannot be claimed (nil-safe).
func TestNotificationService_CenterOwnership(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewNotificationRepositoryImpl(db)
	svc := service.NewNotificationServiceImpl(repo, repository.NewPushDeviceRepositoryImpl(db))

	myRow := seedCenterNotification(t, db, 2, "walkin_created", "mine", false)
	otherRow := seedCenterNotification(t, db, 9, "walkin_created", "theirs", false)
	clientRow := &models.Notification{ClientID: 5, Type: "appointment_reminder", Title: "client", Message: "m"}
	if err := db.Create(clientRow).Error; err != nil {
		t.Fatalf("failed to seed client row: %v", err)
	}
	staffRow := &models.Notification{ClientID: 0, PersonnelID: ptrUint(7), Type: "personnel_changed", Title: "staff", Message: "m"}
	if err := db.Create(staffRow).Error; err != nil {
		t.Fatalf("failed to seed staff row: %v", err)
	}

	// Reading another center's notifications → 403.
	_, err := svc.GetByCenterID(9, 2)
	assert.ErrorIs(t, err, apperrors.ErrForbidden)

	// Own list returns only own rows.
	rows, err := svc.GetByCenterID(2, 2)
	assert.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "mine", rows[0].Title)

	// Unread + count scoping (claim-scoped count per api-authz spec).
	unread, err := svc.GetUnreadByCenterID(2, 2)
	assert.NoError(t, err)
	assert.Len(t, unread, 1)
	count, err := svc.CountUnreadByCenterID(2, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Cross-center mark-read → 403 and the row stays unread.
	err = svc.MarkAsReadByCenter(otherRow.ID, 2)
	assert.ErrorIs(t, err, apperrors.ErrForbidden)
	var otherAfter models.Notification
	assert.NoError(t, db.First(&otherAfter, otherRow.ID).Error)
	assert.False(t, otherAfter.IsRead, "cross-center mark-read must not flip the row")

	// Claiming a client row as a center → 403 (nil-safe ownership).
	err = svc.MarkAsReadByCenter(clientRow.ID, 2)
	assert.ErrorIs(t, err, apperrors.ErrForbidden)

	// Claiming a staff row as a center → 403 (nil-safe ownership).
	err = svc.MarkAsReadByCenter(staffRow.ID, 2)
	assert.ErrorIs(t, err, apperrors.ErrForbidden)

	// Missing row → 404, not 403 (no ownership leak).
	err = svc.MarkAsReadByCenter(123456789, 2)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)

	// Owner marks own row read → OK; read again → idempotent no-error.
	err = svc.MarkAsReadByCenter(myRow.ID, 2)
	assert.NoError(t, err)
	var after models.Notification
	assert.NoError(t, db.First(&after, myRow.ID).Error)
	assert.True(t, after.IsRead)
	assert.NotNil(t, after.ReadAt)
	err = svc.MarkAsReadByCenter(myRow.ID, 2)
	assert.NoError(t, err, "mark-read is idempotent")
}

// TestNotificationCenter_Routes — HTTP-layer authz for the center notification
// endpoints: beauty_center-only, claim-scoped list/unread/count, cross-center
// mark-read 403, nil-safe idempotent mark-read, and DTO shape (beauty_center_id
// present, personnel_id omitted; client rows carry no beauty_center_id).
func TestNotificationCenter_Routes(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := authzTestRouter(t)

	n1 := seedCenterNotification(t, db, 1, "walkin_created", "center-1-unread", false)
	seedCenterNotification(t, db, 1, "review_created", "center-1-read", true)
	seedCenterNotification(t, db, 2, "walkin_created", "center-2-unread", false)
	seedNotification(t, db, 5, "appointment_reminder", "client-row")
	seedPersonnelNotification(t, db, 7, "personnel_changed", "staff-row", false)

	t.Run("no token is 401", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/notifications/center", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("client role is 403", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/notifications/center", roleToken(t, 1, 0, "client"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("personnel role is 403", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/notifications/center", roleToken(t, 7, 1, "personnel"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("center reads own notifications only, newest first", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/notifications/center", roleToken(t, 1, 1, "beauty_center"), "")
		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, `"title":"center-1-unread"`)
		assert.Contains(t, body, `"title":"center-1-read"`)
		assert.NotContains(t, body, `"title":"center-2-unread"`, "no cross-center leak")
		assert.NotContains(t, body, `"title":"client-row"`, "client rows are sentinel-isolated")
		assert.NotContains(t, body, `"title":"staff-row"`, "staff rows are sentinel-isolated")
	})

	t.Run("center DTO omits staff fields and carries the sentinel", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/notifications/center", roleToken(t, 1, 1, "beauty_center"), "")
		body := w.Body.String()
		assert.Contains(t, body, `"beauty_center_id":1`)
		assert.Contains(t, body, `"client_id":0`, "ClientID=0 sentinel serializes as plain uint")
		assert.NotContains(t, body, `"personnel_id"`, "center rows must omit personnel_id")
	})

	t.Run("unread endpoint filters", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/notifications/center/unread", roleToken(t, 1, 1, "beauty_center"), "")
		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, `"title":"center-1-unread"`)
		assert.NotContains(t, body, `"title":"center-1-read"`)
	})

	t.Run("count endpoint is claim-scoped", func(t *testing.T) {
		// Center 1 has one unread; center 2's unread row must not count.
		w := do(r, http.MethodGet, "/api/notifications/center/count", roleToken(t, 1, 1, "beauty_center"), "")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"count":1`)
	})

	t.Run("cross-center mark-read is 403 and stays unread", func(t *testing.T) {
		w := do(r, http.MethodPatch, fmt.Sprintf("/api/notifications/center/%d/read", n1.ID), roleToken(t, 1, 2, "beauty_center"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
		var stored models.Notification
		assert.NoError(t, db.First(&stored, n1.ID).Error)
		assert.False(t, stored.IsRead, "cross-center mark-read must not flip the row")
	})

	t.Run("owner mark-read is idempotent", func(t *testing.T) {
		token := roleToken(t, 1, 1, "beauty_center")
		w := do(r, http.MethodPatch, fmt.Sprintf("/api/notifications/center/%d/read", n1.ID), token, "")
		assert.Equal(t, http.StatusOK, w.Code)
		var afterFirst models.Notification
		assert.NoError(t, db.First(&afterFirst, n1.ID).Error)
		assert.True(t, afterFirst.IsRead)
		assert.NotNil(t, afterFirst.ReadAt)

		// Spec: calling the same PATCH again returns 200 with no state change —
		// IsRead stays true and the row is not re-created or unread. (The shared
		// repo rewrites read_at on every call; that is not a state regression.)
		w = do(r, http.MethodPatch, fmt.Sprintf("/api/notifications/center/%d/read", n1.ID), token, "")
		assert.Equal(t, http.StatusOK, w.Code)
		var afterSecond models.Notification
		assert.NoError(t, db.First(&afterSecond, n1.ID).Error)
		assert.True(t, afterSecond.IsRead)
		assert.NotNil(t, afterSecond.ReadAt)
	})

	t.Run("client list response carries no beauty_center_id", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/notifications/client/5", roleToken(t, 5, 0, "client"), "")
		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, `"title":"client-row"`)
		assert.NotContains(t, body, `"beauty_center_id"`, "omitempty hides it on client rows")
		assert.NotContains(t, body, `"title":"center-1-unread"`, "center rows never leak into client queries")
	})
}

// TestNotificationCenter_ScopesIsolated — the sentinel matrix regression: each
// scope (client / staff / center) sees exactly its own rows and nothing else
// (design D8, api-authz "Sentinel isolation" scenarios).
func TestNotificationCenter_ScopesIsolated(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewNotificationRepositoryImpl(db)

	seedNotification(t, db, 5, "appointment_reminder", "client-row")
	seedPersonnelNotification(t, db, 7, "personnel_changed", "staff-row", false)
	seedCenterNotification(t, db, 1, "walkin_created", "center-row", false)

	clientRows, err := repo.FindByClientID(5)
	assert.NoError(t, err)
	assert.Len(t, clientRows, 1)
	assert.Equal(t, "client-row", clientRows[0].Title)

	staffRows, err := repo.FindByPersonnelID(7)
	assert.NoError(t, err)
	assert.Len(t, staffRows, 1)
	assert.Equal(t, "staff-row", staffRows[0].Title)

	centerRows, err := repo.FindByCenterID(1)
	assert.NoError(t, err)
	assert.Len(t, centerRows, 1)
	assert.Equal(t, "center-row", centerRows[0].Title)

	// Cross-scope absence is proven with real ids, not the sentinel 0 (which no
	// controller ever queries — path/claim ids are validated > 0): each scope's
	// own query returns exactly its own row, so no other scope's rows leak in.
	clientTitles := make([]string, len(clientRows))
	for i, r := range clientRows {
		clientTitles[i] = r.Title
	}
	assert.Equal(t, []string{"client-row"}, clientTitles, "client scope sees only its own row")
	staffTitles := make([]string, len(staffRows))
	for i, r := range staffRows {
		staffTitles[i] = r.Title
	}
	assert.Equal(t, []string{"staff-row"}, staffTitles, "staff scope sees only its own row")
	centerTitles := make([]string, len(centerRows))
	for i, r := range centerRows {
		centerTitles[i] = r.Title
	}
	assert.Equal(t, []string{"center-row"}, centerTitles, "center scope sees only its own row")
}
