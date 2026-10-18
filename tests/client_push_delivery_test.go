package tests

import (
	"sync/atomic"
	"testing"

	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// EVERY client notification must reach the device, not just the ones whose call
// site remembered to ask. Create owns delivery, so a type that used to be
// in-app only (proof_rejected, late_cancel, stamp_earned…) now pushes too — and
// the app can show it while CLOSED.

func TestPush_ClientCreateDeliversToRegisteredDevice(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	withVapid(t)

	var hits int64
	srv := captureWebServer(t, &hits)

	db := setupRouterAuthzDB(t)
	pushRepo := repository.NewPushDeviceRepositoryImpl(db)
	svc := service.NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), pushRepo)

	p256dh, auth := subscriptionKeys(t)
	require.NoError(t, svc.RegisterPushDevice("client", 5, srv.URL+"/client", "web", p256dh, auth))

	// A type that previously wrote ONLY the in-app row.
	require.NoError(t, svc.Create(5, "proof_rejected",
		"Comprobante observado", "Subí uno nuevo antes del vencimiento",
		`{"type":"proof_rejected","reservation_id":1}`))

	require.True(t, waitForCount(t, &hits, 1),
		"Create must push to the client's registered device")
}

func TestPush_ClientCreateNeverCrossesClients(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	withVapid(t)

	var mineHits, otherHits int64
	mineSrv := captureWebServer(t, &mineHits)
	otherSrv := captureWebServer(t, &otherHits)

	db := setupRouterAuthzDB(t)
	pushRepo := repository.NewPushDeviceRepositoryImpl(db)
	svc := service.NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), pushRepo)

	p256dh, auth := subscriptionKeys(t)
	require.NoError(t, svc.RegisterPushDevice("client", 5, mineSrv.URL+"/mine", "web", p256dh, auth))
	require.NoError(t, svc.RegisterPushDevice("client", 6, otherSrv.URL+"/other", "web", p256dh, auth))

	require.NoError(t, svc.Create(5, "late_cancel", "Cancelación tardía", "Tu confianza bajó", `{"type":"late_cancel"}`))

	require.True(t, waitForCount(t, &mineHits, 1))
	assert.Equal(t, int64(0), atomic.LoadInt64(&otherHits), "another client must not be notified")
}

// TestPush_CreateWithoutDevicesIsSilent: a client who never granted permission
// must not turn a booking/cancellation into an error.
func TestPush_CreateWithoutDevicesIsSilent(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	withVapid(t)

	db := setupRouterAuthzDB(t)
	svc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)

	require.NoError(t, svc.Create(5, "reminder_24h", "Recordatorio", "Tenés turno mañana", `{}`),
		"no registered device must be a no-op, never an error")
}
