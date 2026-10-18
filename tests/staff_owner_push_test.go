package tests

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Staff / owner push (role-aware devices).
//
// A push device is keyed by (role, owner). That scoping is the WHOLE point: ids
// are per-table, so client #7 and professional #7 exist simultaneously. A
// lookup that ignored the role would leak one person's notification to another.

// captureWebServer returns a server that counts the pushes it receives.
func captureWebServer(t *testing.T, counter *int64) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(counter, 1)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// waitForCount polls until the counter reaches `want` or the deadline passes,
// because the web push is dispatched synchronously but the HTTP round-trip is
// not ordered against the counter read on a slow machine.
func waitForCount(t *testing.T, counter *int64, want int64) bool {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(counter) >= want {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func TestPush_StaffNotificationReachesOnlyStaffDevices(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	withVapid(t)

	var staffHits, clientHits int64
	staffSrv := captureWebServer(t, &staffHits)
	clientSrv := captureWebServer(t, &clientHits)

	db := setupRouterAuthzDB(t)
	pushRepo := repository.NewPushDeviceRepositoryImpl(db)
	svc := service.NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), pushRepo)

	// BOTH device owners have id 7 — one as a professional, one as a client.
	p256dh, auth := subscriptionKeys(t)
	require.NoError(t, svc.RegisterPushDevice("personnel", 7, staffSrv.URL+"/staff", "web", p256dh, auth))
	require.NoError(t, svc.RegisterPushDevice("client", 7, clientSrv.URL+"/client", "web", p256dh, auth))

	require.NoError(t, svc.CreateForPersonnel(7, "reservation_cancelled", "Turno cancelado", "Tu turno fue cancelado", `{"reservation_id":9}`))

	require.True(t, waitForCount(t, &staffHits, 1), "the professional's device must receive the push")
	assert.Equal(t, int64(0), atomic.LoadInt64(&clientHits),
		"client #7 must NOT receive professional #7's notification")
}

func TestPush_CentreNotificationReachesOnlyCentreDevices(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	withVapid(t)

	var centreHits, clientHits int64
	centreSrv := captureWebServer(t, &centreHits)
	clientSrv := captureWebServer(t, &clientHits)

	db := setupRouterAuthzDB(t)
	pushRepo := repository.NewPushDeviceRepositoryImpl(db)
	svc := service.NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), pushRepo)

	p256dh, auth := subscriptionKeys(t)
	require.NoError(t, svc.RegisterPushDevice("beauty_center", 7, centreSrv.URL+"/centre", "web", p256dh, auth))
	require.NoError(t, svc.RegisterPushDevice("client", 7, clientSrv.URL+"/client", "web", p256dh, auth))

	require.NoError(t, svc.CreateForCenter(7, "review_created", "Nueva reseña", "Un cliente dejó una reseña", `{}`))

	require.True(t, waitForCount(t, &centreHits, 1), "the centre's device must receive the push")
	assert.Equal(t, int64(0), atomic.LoadInt64(&clientHits),
		"client #7 must NOT receive centre #7's notification")
}

// TestPush_NotificationWriteSurvivesPushFailure: the in-app row is the contract;
// a broken push transport must never fail the operation that produced it.
func TestPush_NotificationWriteSurvivesPushFailure(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	withVapid(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	db := setupRouterAuthzDB(t)
	pushRepo := repository.NewPushDeviceRepositoryImpl(db)
	svc := service.NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), pushRepo)

	p256dh, auth := subscriptionKeys(t)
	require.NoError(t, svc.RegisterPushDevice("personnel", 7, srv.URL+"/broken", "web", p256dh, auth))

	require.NoError(t, svc.CreateForPersonnel(7, "personnel_changed", "Cambio", "Te reasignaron un turno", `{}`),
		"a failing push must not fail the notification write")
}
