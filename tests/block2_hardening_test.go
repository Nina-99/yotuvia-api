package tests

// Block 2 regression coverage:
//   - the HTTP server has timeouts (Slowloris) — asserted on the real config;
//   - a non-client actor cannot read a client's notification feed through the
//     polymorphic user-id collision.

import (
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHTTPServer_HasSlowlorisTimeouts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := router.NewHTTPServer(":0", gin.New())
	t.Cleanup(func() { _ = srv.Close() })

	assert.Positive(t, srv.ReadHeaderTimeout,
		"without ReadHeaderTimeout a client can hold a connection open forever with a partial header")
	assert.Positive(t, srv.ReadTimeout)
	assert.Positive(t, srv.WriteTimeout)
	assert.Positive(t, srv.IdleTimeout)
	assert.Equal(t, 1<<20, srv.MaxHeaderBytes)
}

// TestNotificationClientRoutes_RejectNonClientActors — user ids are
// polymorphic: client #5, personnel #5 and centre #5 share the numeric space.
// The service only compares numbers, so the ROLE gate is what stops a staff or
// centre token from reading a client's notifications.
func TestNotificationClientRoutes_RejectNonClientActors(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	gin.SetMode(gin.TestMode)

	db := setupRouterAuthzDB(t)
	r := gin.New()
	router.NotificationRoutes(r.Group("/api"), db)

	notif := models.Notification{
		ClientID: 5, Title: "Turno confirmado", Message: "Sos client 5",
	}
	require.NoError(t, db.Create(&notif).Error)

	t.Run("client reads its own feed", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/notifications/client/5", roleToken(t, 5, 0, "client"), "")
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "Turno confirmado")
	})

	t.Run("personnel with the same numeric id is forbidden", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/notifications/client/5", roleToken(t, 5, 1, "personnel"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.NotContains(t, w.Body.String(), "Turno confirmado")
	})

	t.Run("centre with the same numeric id is forbidden", func(t *testing.T) {
		w := do(r, http.MethodGet, "/api/notifications/client/5", roleToken(t, 5, 5, "beauty_center"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.NotContains(t, w.Body.String(), "Turno confirmado")
	})

	t.Run("personnel cannot mark a client notification read", func(t *testing.T) {
		w := do(r, http.MethodPatch, "/api/notifications/1/read", roleToken(t, 5, 1, "personnel"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
