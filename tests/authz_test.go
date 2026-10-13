package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// seedReservation inserts a reservation row directly (bypassing the create
// flow) so ownership tests control ClientID/BeautyCenterID explicitly.
func seedReservation(t *testing.T, db *gorm.DB, clientID, centerID uint) *models.Reservation {
	t.Helper()
	res := &models.Reservation{
		BeautyCenterID: centerID,
		ClientID:       clientID,
		PersonnelID:    1,
		Status:         "PENDING",
		TotalPrice:     100,
	}
	if err := db.Create(res).Error; err != nil {
		t.Fatalf("failed to seed reservation: %v", err)
	}
	return res
}

// TestReservationOwnership_CrossOwnerAccess — cross-owner reads/writes on the
// /:id family must be 403 regardless of role; owners must still succeed.
func TestReservationOwnership_CrossOwnerAccess(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	t.Run("cross-owner GET /:id is 403 and leaks no reservation data", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedReservation(t, db, 1, 1)

		foreign := roleToken(t, 999, 0, "client") // owns nothing
		w := do(r, http.MethodGet, "/api/reservation/1", foreign, "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
		assert.NotContains(t, w.Body.String(), `"total_price":100`)
	})

	t.Run("owner client reads own reservation", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedReservation(t, db, 1, 1)

		owner := roleToken(t, 1, 1, "client")
		w := do(r, http.MethodGet, "/api/reservation/1", owner, "")

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("owner center reads reservation", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedReservation(t, db, 1, 1)

		admin := roleToken(t, 7, 1, "beauty_center")
		w := do(r, http.MethodGet, "/api/reservation/1", admin, "")

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("cross-owner PUT /:id is 403", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedReservation(t, db, 1, 1)

		foreign := roleToken(t, 999, 0, "client")
		body := `{"beauty_center_id":1,"personnel_id":1,"service_ids":[1],"date":"2026-08-01","start_time":"10:00"}`
		w := do(r, http.MethodPut, "/api/reservation/1", foreign, body)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("owner client updates own reservation", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedReservation(t, db, 1, 1)
		// The update recalculates prices from the service catalog.
		if err := db.Create(&models.Service{
			BeautyCenterID: 1, CategoryID: 1, Name: "Corte",
			Description: "Corte de pelo", Duration: 30, Price: 50,
		}).Error; err != nil {
			t.Fatalf("failed to seed service: %v", err)
		}

		owner := roleToken(t, 1, 1, "client")
		body := `{"beauty_center_id":1,"personnel_id":1,"service_ids":[1],"date":"2026-08-01","start_time":"10:00"}`
		w := do(r, http.MethodPut, "/api/reservation/1", owner, body)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("cross-owner DELETE /:id is 403", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedReservation(t, db, 1, 1)

		foreign := roleToken(t, 999, 0, "client")
		w := do(r, http.MethodDelete, "/api/reservation/1", foreign, "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("owner client deletes own reservation", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedReservation(t, db, 1, 1)

		owner := roleToken(t, 1, 1, "client")
		w := do(r, http.MethodDelete, "/api/reservation/1", owner, "")

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("foreign generate-qr is 403 and issues no token", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedReservation(t, db, 1, 1)

		foreign := roleToken(t, 999, 0, "client")
		w := do(r, http.MethodPost, "/api/reservation/generate-qr/1", foreign, "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
		assert.NotContains(t, w.Body.String(), `"token":`)
	})

	t.Run("owner client can generate QR", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedReservation(t, db, 1, 1)

		owner := roleToken(t, 1, 1, "client")
		w := do(r, http.MethodPost, "/api/reservation/generate-qr/1", owner, "")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"token":`)
	})

	t.Run("client PATCH /status is 403 and status is unchanged", func(t *testing.T) {
		r, db := authzTestRouter(t)
		res := seedReservation(t, db, 1, 1)

		client := roleToken(t, 1, 1, "client")
		w := do(r, http.MethodPatch, "/api/reservation/1/status", client, `{"status":"CONFIRMED"}`)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))

		var stored models.Reservation
		if err := db.First(&stored, res.ID).Error; err != nil {
			t.Fatalf("failed to reload reservation: %v", err)
		}
		assert.Equal(t, "PENDING", stored.Status)
	})
}

// TestReservationFindAll_ScopedToCenterClaim — the beauty_center token must only
// ever see its own center's reservations, never the global list.
func TestReservationFindAll_ScopedToCenterClaim(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := authzTestRouter(t)
	center1 := seedReservation(t, db, 1, 1)
	center2 := seedReservation(t, db, 2, 2)

	admin := roleToken(t, 7, 1, "beauty_center") // owns center 1
	w := do(r, http.MethodGet, "/api/reservation", admin, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data []struct {
			ID             uint `json:"id"`
			BeautyCenterID uint `json:"beauty_center_id"`
		} `json:"data"`
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	assert.Equal(t, int64(1), body.Total)
	assert.Len(t, body.Data, 1)
	assert.Equal(t, center1.ID, body.Data[0].ID)
	assert.Equal(t, uint(1), body.Data[0].BeautyCenterID)
	assert.NotEqual(t, center2.ID, body.Data[0].ID)
}

// TestReservationCenterStaff_ReadsOwnCenterOnly — personnel may read their own
// center's reservations, never another center's.
func TestReservationCenterStaff_ReadsOwnCenterOnly(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)

	t.Run("personnel reads own center", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedReservation(t, db, 1, 1)

		staff := roleToken(t, 7, 1, "personnel")
		w := do(r, http.MethodGet, "/api/reservation/center/1", staff, "")

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("personnel cannot read another center", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedReservation(t, db, 1, 1) // center 1
		seedReservation(t, db, 2, 2) // center 2

		staff := roleToken(t, 7, 1, "personnel")
		w := do(r, http.MethodGet, "/api/reservation/center/2", staff, "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("beauty_center reads own center", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedReservation(t, db, 1, 1)

		admin := roleToken(t, 7, 1, "beauty_center")
		w := do(r, http.MethodGet, "/api/reservation/center/1", admin, "")

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestReservationClientScopedReads — the /client/:clientId routes must reject a
// path that does not match the caller's clientID claim.
func TestReservationClientScopedReads(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)

	t.Run("client reads own reservation list", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedReservation(t, db, 1, 1)

		owner := roleToken(t, 1, 1, "client")
		w := do(r, http.MethodGet, "/api/reservation/client/1", owner, "")

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("client path/claim mismatch is 403", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedReservation(t, db, 1, 1)

		owner := roleToken(t, 1, 1, "client") // claim 1, path 9
		w := do(r, http.MethodGet, "/api/reservation/client/9", owner, "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})
}

// seedClient inserts a client row with an explicit ID so /client/:id ownership
// tests can target a known profile.
func seedClient(t *testing.T, db *gorm.DB, id uint, username, email string) *models.Client {
	t.Helper()
	client := &models.Client{
		Model:     gorm.Model{ID: id},
		FirstName: "Test",
		LastName:  "Client",
		Username:  username,
		Phone:     "5550001",
		Email:     email,
		Password:  "hashed",
		IsActive:  true,
	}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("failed to seed client: %v", err)
	}
	return client
}

// seedPersonnel inserts a personnel row with an explicit ID.
func seedPersonnel(t *testing.T, db *gorm.DB, id, centerID uint) *models.Personnel {
	t.Helper()
	p := &models.Personnel{
		Model:          gorm.Model{ID: id},
		BeautyCenterID: centerID,
		FirstName:      "Jane",
		LastName:       "Doe",
		Email:          "jane.doe@example.com",
		Phone:          "5550002",
		IsActive:       true,
	}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("failed to seed personnel: %v", err)
	}
	return p
}

// seedNotification inserts a notification row for the given client.
func seedNotification(t *testing.T, db *gorm.DB, clientID uint, notifType, title string) *models.Notification {
	t.Helper()
	n := &models.Notification{
		ClientID: clientID,
		Type:     notifType,
		Title:    title,
		Message:  "message",
	}
	if err := db.Create(n).Error; err != nil {
		t.Fatalf("failed to seed notification: %v", err)
	}
	return n
}

// TestClientOwnership_ProfileMismatch — GET/PUT/DELETE /client/:id are
// owner-only: any path :id that differs from the caller's clientID claim is 403
// and must not read or mutate the profile.
func TestClientOwnership_ProfileMismatch(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	t.Run("client reads own profile", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedClient(t, db, 1, "nina", "nina@test.com")

		owner := roleToken(t, 1, 0, "client")
		w := do(r, http.MethodGet, "/api/client/1", owner, "")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"username":"nina"`)
	})

	t.Run("cross-client GET /client/:id is 403 and leaks no profile", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedClient(t, db, 1, "nina", "nina@test.com")

		other := roleToken(t, 9, 0, "client") // claim 9, path 1
		w := do(r, http.MethodGet, "/api/client/1", other, "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
		assert.NotContains(t, w.Body.String(), `"username":"nina"`)
	})

	t.Run("cross-client PUT /client/:id is 403 and profile is unchanged", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedClient(t, db, 1, "nina", "nina@test.com")

		other := roleToken(t, 9, 0, "client")
		body := `{"first_name":"Hacked","last_name":"Hacked","username":"hacked","email":"hacked@test.com","password":"secret123","phone":"123","is_active":true}`
		w := do(r, http.MethodPut, "/api/client/1", other, body)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))

		var stored models.Client
		if err := db.First(&stored, 1).Error; err != nil {
			t.Fatalf("failed to reload client: %v", err)
		}
		assert.Equal(t, "nina", stored.Username)
	})

	t.Run("cross-client DELETE /client/:id is 403 and profile survives", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedClient(t, db, 1, "nina", "nina@test.com")

		other := roleToken(t, 9, 0, "client")
		w := do(r, http.MethodDelete, "/api/client/1", other, "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))

		var stored models.Client
		if err := db.First(&stored, 1).Error; err != nil {
			t.Fatalf("cross-client DELETE removed the profile: %v", err)
		}
	})
}

// TestPersonnelClientEditForbidden — personnel routes are JWT-protected (no
// token → 401) and write handlers are beauty_center-only (client → 403).
func TestPersonnelClientEditForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	t.Run("personnel route without token is 401", func(t *testing.T) {
		r, _ := authzTestRouter(t)
		w := do(r, http.MethodGet, "/api/personnel/1", "", "")

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("client PUT /personnel/:id is 403", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedPersonnel(t, db, 1, 1)

		client := roleToken(t, 1, 0, "client")
		body := `{"first_name":"X","phone":"555"}`
		w := do(r, http.MethodPut, "/api/personnel/1", client, body)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("client DELETE /personnel/:id is 403", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedPersonnel(t, db, 1, 1)

		client := roleToken(t, 1, 0, "client")
		w := do(r, http.MethodDelete, "/api/personnel/1", client, "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})
}

// TestNotificationOwnership_CrossClient — notifications are client-scoped: the
// path clientId must match the actor's claim and read/read-all must only touch
// the actor's own notifications.
func TestNotificationOwnership_CrossClient(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	t.Run("notification route without token is 401", func(t *testing.T) {
		r, _ := authzTestRouter(t)
		w := do(r, http.MethodGet, "/api/notifications/client/1", "", "")

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("client reads own notifications", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedNotification(t, db, 1, "reservation", "hello-nina")

		owner := roleToken(t, 1, 0, "client")
		w := do(r, http.MethodGet, "/api/notifications/client/1", owner, "")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"title":"hello-nina"`)
	})

	t.Run("cross-client GET /notifications/client/:clientId is 403", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedNotification(t, db, 2, "reservation", "secret-title")

		other := roleToken(t, 9, 0, "client") // claim 9, path 2
		w := do(r, http.MethodGet, "/api/notifications/client/2", other, "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
		assert.NotContains(t, w.Body.String(), `"secret-title"`)
	})

	t.Run("cross-client PATCH /notifications/:id/read is 403", func(t *testing.T) {
		r, db := authzTestRouter(t)
		n := seedNotification(t, db, 2, "reservation", "for-others")

		other := roleToken(t, 1, 0, "client") // actor 1, notification belongs to client 2
		w := do(r, http.MethodPatch, fmt.Sprintf("/api/notifications/%d/read", n.ID), other, "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
	})

	t.Run("owner marks own notification read", func(t *testing.T) {
		r, db := authzTestRouter(t)
		n := seedNotification(t, db, 1, "reservation", "for-me")

		owner := roleToken(t, 1, 0, "client")
		w := do(r, http.MethodPatch, fmt.Sprintf("/api/notifications/%d/read", n.ID), owner, "")

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
