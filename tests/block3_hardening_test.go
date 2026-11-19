package tests

// Block 3 regression coverage: the disk floor that protects the volume Postgres
// shares with the uploads, and the ownership check on push unregistration.

import (
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/middleware"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestDiskGuard_RefusesWhenTheVolumeIsFull — the floor is what stops a disk-full
// outage: on a single-volume VPS a full /uploads takes Postgres down with it.
func TestDiskGuard_RefusesWhenTheVolumeIsFull(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()

	t.Run("an unreachable floor rejects the write with 503", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		// 1 EiB free is never available, so the guard must trip.
		r.POST("/upload", middleware.DiskGuard(1<<60), func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{"ok": true})
		})

		w := do(r, http.MethodPost, "/upload", "", "")
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		assert.Contains(t, w.Body.String(), "not enough storage")
	})

	t.Run("a floor of zero disables the check", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.POST("/upload", middleware.DiskGuard(0), func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{"ok": true})
		})

		w := do(r, http.MethodPost, "/upload", "", "")
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("a realistic floor lets the write through", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.POST("/upload", middleware.DiskGuard(1<<20), func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{"ok": true})
		})

		w := do(r, http.MethodPost, "/upload", "", "")
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestPushUnregister_IsOwnerScoped — the endpoint already required a JWT, but it
// deleted by token alone: any authenticated caller who knew a subscription
// endpoint could unsubscribe someone else's device.
func TestPushUnregister_IsOwnerScoped(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	gin.SetMode(gin.TestMode)

	db := setupRouterAuthzDB(t)
	r := gin.New()
	router.NotificationRoutes(r.Group("/api"), db)

	// One device owned by client 5.
	device := models.PushDevice{
		ClientID:  5,
		OwnerRole: "client",
		PushToken: "https://push.example/sub/client-5",
		Platform:  "web",
	}
	require.NoError(t, db.Create(&device).Error)

	body := `{"push_token":"https://push.example/sub/client-5","platform":"web"}`

	t.Run("a same-numbered staff id cannot unsubscribe it", func(t *testing.T) {
		w := do(r, http.MethodPost, "/api/push-devices/unregister", roleToken(t, 5, 1, "personnel"), body)
		// Either forbidden or a no-op is acceptable; the device must survive.
		var count int64
		require.NoError(t, db.Model(&models.PushDevice{}).
			Where("push_token = ?", device.PushToken).Count(&count).Error)
		assert.Equal(t, int64(1), count, "another owner's device must not be deleted (got %d)", w.Code)
	})

	t.Run("the owner can unsubscribe it", func(t *testing.T) {
		w := do(r, http.MethodPost, "/api/push-devices/unregister", roleToken(t, 5, 0, "client"), body)
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

		var count int64
		require.NoError(t, db.Model(&models.PushDevice{}).
			Where("push_token = ?", device.PushToken).Count(&count).Error)
		assert.Zero(t, count)
	})
}
