package tests

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Web Push (Phase C4). A browser subscription is stored alongside the native
// Expo tokens and is delivered through RFC 8291 encryption with VAPID.
//
// The suite pins the whole contract: registration fails closed without VAPID
// configuration or without the subscription keys, native registration is
// untouched, the application server key is public, and the send path really hits
// the subscription endpoint (never Expo) — pruning subscriptions the push
// service reports as gone.

func webPushRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	r := gin.New()
	api := r.Group("/api")
	router.NotificationRoutes(api, db)
	return r, db
}

// withoutVapid clears the Web Push configuration for the test.
func withoutVapid(t *testing.T) {
	t.Helper()
	t.Setenv("VAPID_PUBLIC_KEY", "")
	t.Setenv("VAPID_PRIVATE_KEY", "")
}

// withVapid installs a REAL key pair (the same generator the library exposes).
func withVapid(t *testing.T) (public string) {
	t.Helper()
	private, pub, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	t.Setenv("VAPID_PUBLIC_KEY", pub)
	t.Setenv("VAPID_PRIVATE_KEY", private)
	t.Setenv("VAPID_SUBJECT", "mailto:test@yotuvia.app")
	return pub
}

// subscriptionKeys generates the pair a browser would send: an uncompressed
// P-256 point and a 16-byte auth secret, both base64url.
func subscriptionKeys(t *testing.T) (p256dh, auth string) {
	t.Helper()
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	point := elliptic.Marshal(elliptic.P256(), private.PublicKey.X, private.PublicKey.Y)
	secret := make([]byte, 16)
	_, err = rand.Read(secret)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(point), base64.RawURLEncoding.EncodeToString(secret)
}

func webRegisterBody(endpoint, p256dh, auth string) string {
	return fmt.Sprintf(
		`{"push_token":%q,"platform":"web","p256dh":%q,"auth":%q}`,
		endpoint, p256dh, auth,
	)
}

func TestWebPush_RegisterWebFailsClosedWithoutVapid(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	withoutVapid(t)

	r, db := webPushRouter(t)
	seedDashboardClient(t, db, 1, "Ana", "Lopez")
	p256dh, auth := subscriptionKeys(t)

	w := do(r, http.MethodPost, "/api/push-devices/register", roleToken(t, 1, 0, "client"),
		webRegisterBody("https://push.example/sub/1", p256dh, auth))
	assert.Equal(t, http.StatusBadRequest, w.Code,
		"a web subscription cannot be stored when the server has no VAPID keys")

	var count int64
	require.NoError(t, db.Model(&models.PushDevice{}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "nothing is persisted")
}

func TestWebPush_RegisterWebRequiresSubscriptionKeys(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	withVapid(t)

	r, db := webPushRouter(t)
	seedDashboardClient(t, db, 1, "Ana", "Lopez")

	// Keys omitted → the subscription could never be encrypted.
	w := do(r, http.MethodPost, "/api/push-devices/register", roleToken(t, 1, 0, "client"),
		`{"push_token":"https://push.example/sub/2","platform":"web"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	p256dh, auth := subscriptionKeys(t)
	w = do(r, http.MethodPost, "/api/push-devices/register", roleToken(t, 1, 0, "client"),
		webRegisterBody("https://push.example/sub/3", p256dh, auth))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var stored models.PushDevice
	require.NoError(t, db.Where("push_token = ?", "https://push.example/sub/3").First(&stored).Error)
	assert.Equal(t, "web", stored.Platform)
	require.NotNil(t, stored.P256dh)
	require.NotNil(t, stored.Auth)
}

func TestWebPush_RegisterNativeUnaffectedByMissingVapid(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	withoutVapid(t)

	r, db := webPushRouter(t)
	seedDashboardClient(t, db, 1, "Ana", "Lopez")

	w := do(r, http.MethodPost, "/api/push-devices/register", roleToken(t, 1, 0, "client"),
		`{"push_token":"ExponentPushToken[abc]","platform":"ios"}`)
	assert.Equal(t, http.StatusCreated, w.Code,
		"native registration must not depend on the Web Push configuration: %s", w.Body.String())
}

func TestWebPush_VapidPublicKeyIsPublic(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	public := withVapid(t)

	r, _ := webPushRouter(t)

	// No Authorization header on purpose: the browser needs the key before it
	// can subscribe, so the endpoint cannot sit behind the JWT.
	w := do(r, http.MethodGet, "/api/push-devices/vapid-public-key", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), public)
}

// TestWebPush_DeliversToTheSubscriptionEndpoint proves the whole web branch:
// the payload goes to the subscription URL — never Expo — and carries the Web
// Push encryption/VAPID headers.
func TestWebPush_DeliversToTheSubscriptionEndpoint(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	withVapid(t)

	received := make(chan *http.Request, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	db := setupRouterAuthzDB(t)
	pushRepo := repository.NewPushDeviceRepositoryImpl(db)
	svc := service.NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), pushRepo)

	seedDashboardClient(t, db, 1, "Ana", "Lopez")
	p256dh, auth := subscriptionKeys(t)
	require.NoError(t, svc.RegisterPushDevice(1, srv.URL+"/push/1", "web", p256dh, auth))

	require.NoError(t, svc.SendPushToClient(1, "Turno confirmado", "Tu turno está listo", `{"reservation_id":9}`))

	select {
	case req := <-received:
		assert.Equal(t, "aes128gcm", req.Header.Get("Content-Encoding"),
			"the payload must be Web Push encrypted")
		assert.Contains(t, req.Header.Get("Authorization"), "vapid ",
			"the request must be VAPID-signed")
	case <-time.After(5 * time.Second):
		t.Fatal("the subscription endpoint never received the push")
	}
}

// TestWebPush_PrunesGoneSubscription: a 410 means the browser dropped the
// subscription, so the row is deleted instead of being retried forever.
func TestWebPush_PrunesGoneSubscription(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	withVapid(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()

	db := setupRouterAuthzDB(t)
	pushRepo := repository.NewPushDeviceRepositoryImpl(db)
	svc := service.NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), pushRepo)

	seedDashboardClient(t, db, 1, "Ana", "Lopez")
	p256dh, auth := subscriptionKeys(t)
	require.NoError(t, svc.RegisterPushDevice(1, srv.URL+"/push/gone", "web", p256dh, auth))

	require.NoError(t, svc.SendPushToClient(1, "t", "b", ""))

	var count int64
	require.NoError(t, db.Model(&models.PushDevice{}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "a gone subscription is pruned")
}
