package tests

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/middleware"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

// limiterBaseConfig starts from the production defaults and then opens both
// dimensions wide, so each test only tightens the dimension it exercises.
func limiterBaseConfig() middleware.RateLimitConfig {
	cfg := middleware.DefaultRateLimitConfig()
	cfg.IPRate = rate.Limit(60.0 / 60.0)
	cfg.IPBurst = 100
	cfg.IdentifierRate = rate.Limit(60.0 / 60.0)
	cfg.IdentifierBurst = 100
	return cfg
}

// newLimiterRouter mounts the limiter on a fresh engine whose probe handler
// echoes the body back, so tests can assert the body survived the middleware.
func newLimiterRouter(t *testing.T, cfg middleware.RateLimitConfig) (*gin.Engine, *middleware.RateLimiter) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rl := middleware.NewRateLimiter(cfg)
	r := gin.New()
	r.POST("/probe", rl.Handler(), func(ctx *gin.Context) {
		body, _ := io.ReadAll(ctx.Request.Body)
		ctx.JSON(http.StatusOK, gin.H{"body": string(body)})
	})
	return r, rl
}

// limiterRequest fires a probe with an explicit RemoteAddr and optional spoofed
// X-Forwarded-For, so IP/identifier behaviour is deterministic.
func limiterRequest(r *gin.Engine, remoteAddr, xff, body string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(http.MethodPost, "/probe", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func assert429(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "body=%s", w.Body.String())
	assert.Equal(t, "too many requests", errBody(t, w.Body.String()))
	raw := w.Header().Get("Retry-After")
	assert.NotEmpty(t, raw, "throttle must carry Retry-After")
	n, err := strconv.Atoi(raw)
	assert.NoError(t, err, "Retry-After must be integer seconds")
	assert.GreaterOrEqual(t, n, 1)
}

// TestRateLimit_PerIP — exceeding the per-IP burst throttles with 429 +
// Retry-After; a different IP and a different identifier are unaffected.
func TestRateLimit_PerIP(t *testing.T) {
	cfg := limiterBaseConfig()
	cfg.IPRate = rate.Limit(1.0 / 60.0) // one refill per minute
	cfg.IPBurst = 2
	r, _ := newLimiterRouter(t, cfg)

	body := `{"identifier":"a@test.com"}`
	for i := 0; i < 2; i++ {
		w := limiterRequest(r, "10.0.0.1:1111", "", body)
		assert.Equal(t, http.StatusOK, w.Code, "attempt %d under burst must pass", i+1)
	}

	assert429(t, limiterRequest(r, "10.0.0.1:1111", "", body))

	// A different IP has its own bucket.
	w := limiterRequest(r, "10.0.0.2:2222", "", body)
	assert.Equal(t, http.StatusOK, w.Code, "a different IP must be unaffected")

	// The throttled IP is not blocked by a different identifier either: the burst
	// is already spent, so this asserts the IP dimension is what tripped.
	w = limiterRequest(r, "10.0.0.1:1111", "", `{"identifier":"other@test.com"}`)
	assert429(t, w)
}

// TestRateLimit_PerIdentifierAcrossIPs — a distributed attack on ONE account is
// throttled by the per-identifier dimension even when every request comes from a
// different IP; a different identifier from yet another IP is unaffected.
func TestRateLimit_PerIdentifierAcrossIPs(t *testing.T) {
	cfg := limiterBaseConfig()
	cfg.IdentifierRate = rate.Limit(1.0 / 60.0)
	cfg.IdentifierBurst = 2
	r, _ := newLimiterRouter(t, cfg)

	victim := `{"identifier":"victim@test.com"}`
	assert.Equal(t, http.StatusOK, limiterRequest(r, "1.1.1.1:1", "", victim).Code)
	assert.Equal(t, http.StatusOK, limiterRequest(r, "2.2.2.2:2", "", victim).Code)

	// Third IP, same account → throttled.
	assert429(t, limiterRequest(r, "3.3.3.3:3", "", victim))

	// Different identifier → its own budget (note "email" is also an identifier
	// key the limiter extracts from the body).
	w := limiterRequest(r, "4.4.4.4:4", "", `{"email":"other@test.com"}`)
	assert.Equal(t, http.StatusOK, w.Code, "a different identifier must be unaffected")
}

// TestRateLimit_BodyPreserved — the handler still receives an intact body, for
// both JSON and non-JSON payloads.
func TestRateLimit_BodyPreserved(t *testing.T) {
	cfg := limiterBaseConfig()
	r, _ := newLimiterRouter(t, cfg)

	jsonBody := `{"identifier":"x@test.com","password":"s3cret-value"}`
	w := limiterRequest(r, "10.0.0.9:1", "", jsonBody)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "s3cret-value", "handler must see the intact JSON body")

	w = limiterRequest(r, "10.0.0.9:1", "", "this-is-not-json")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "this-is-not-json", "non-JSON body must survive untouched")
}

// TestRateLimit_OversizedBodySkipsIdentifier — a body larger than MaxBodyBytes
// skips only the identifier dimension and is still delivered in full.
func TestRateLimit_OversizedBodySkipsIdentifier(t *testing.T) {
	cfg := limiterBaseConfig()
	cfg.MaxBodyBytes = 16
	cfg.IdentifierRate = rate.Limit(1.0 / 3600.0)
	cfg.IdentifierBurst = 1
	r, _ := newLimiterRouter(t, cfg)

	big := fmt.Sprintf(`{"identifier":"%s"}`, strings.Repeat("x", 100))
	for i := 0; i < 3; i++ {
		w := limiterRequest(r, "10.0.0.9:1", "", big)
		assert.Equal(t, http.StatusOK, w.Code,
			"oversized body must skip the identifier dimension (attempt %d)", i+1)
		assert.Contains(t, w.Body.String(), strings.Repeat("x", 100),
			"oversized body must still reach the handler in full")
	}

	// Control: the same config with a small body IS throttled on the second call,
	// proving the previous requests were skipped because of size, not because the
	// identifier dimension was inert.
	small := `{"email":"y@t"}` // 15 bytes: under the 16-byte MaxBodyBytes cap
	assert.Equal(t, http.StatusOK, limiterRequest(r, "10.0.0.9:1", "", small).Code)
	assert429(t, limiterRequest(r, "10.0.0.9:1", "", small))
}

// TestRateLimit_SpoofedXFFDoesNotBuyBudget — with TRUSTED_PROXIES unset, an
// arbitrary client-supplied X-Forwarded-For MUST NOT grant a fresh per-IP
// budget: ClientIP falls back to the TCP RemoteAddr.
func TestRateLimit_SpoofedXFFDoesNotBuyBudget(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	cfg := limiterBaseConfig()
	cfg.IPRate = rate.Limit(1.0 / 60.0)
	cfg.IPBurst = 2
	cfg.IdentifierKeys = nil // isolate the per-IP dimension

	gin.SetMode(gin.TestMode)
	rl := middleware.NewRateLimiter(cfg)
	r := gin.New()
	router.ConfigureTrustedProxies(r)
	r.POST("/probe", rl.Handler(), func(ctx *gin.Context) { ctx.Status(http.StatusOK) })

	assert.Equal(t, http.StatusOK, limiterRequest(r, "10.0.0.1:1", "1.1.1.1", "").Code)
	assert.Equal(t, http.StatusOK, limiterRequest(r, "10.0.0.1:1", "2.2.2.2", "").Code)
	// Rotating the spoofed header does NOT buy a third token.
	assert429(t, limiterRequest(r, "10.0.0.1:1", "3.3.3.3", ""))
}

// TestRateLimit_TrustedProxyHonoured — when TRUSTED_PROXIES includes the
// connecting proxy, X-Forwarded-For IS trusted and each forwarded client gets
// its own budget.
func TestRateLimit_TrustedProxyHonoured(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")
	cfg := limiterBaseConfig()
	cfg.IPRate = rate.Limit(1.0 / 60.0)
	cfg.IPBurst = 2
	cfg.IdentifierKeys = nil

	gin.SetMode(gin.TestMode)
	rl := middleware.NewRateLimiter(cfg)
	r := gin.New()
	router.ConfigureTrustedProxies(r)
	r.POST("/probe", rl.Handler(), func(ctx *gin.Context) { ctx.Status(http.StatusOK) })

	// Same TCP peer (the trusted proxy), distinct forwarded clients → distinct
	// buckets.
	assert.Equal(t, http.StatusOK, limiterRequest(r, "127.0.0.1:5000", "9.9.9.9", "").Code)
	assert.Equal(t, http.StatusOK, limiterRequest(r, "127.0.0.1:5000", "9.9.9.9", "").Code)
	assert429(t, limiterRequest(r, "127.0.0.1:5000", "9.9.9.9", ""))
	assert.Equal(t, http.StatusOK, limiterRequest(r, "127.0.0.1:5000", "8.8.8.8", "").Code,
		"a different forwarded client through the trusted proxy has its own budget")
}

// TestRateLimit_EvictionBoundsMemory — the key maps are bounded by a hard cap
// and by an idle TTL sweep; neither dimension grows without limit.
func TestRateLimit_EvictionBoundsMemory(t *testing.T) {
	t.Run("hard cap bounds the map", func(t *testing.T) {
		cfg := limiterBaseConfig()
		cfg.IPRate = rate.Limit(1_000_000)
		cfg.IPBurst = 1_000_000
		cfg.IdentifierKeys = nil
		cfg.MaxEntries = 5
		r, rl := newLimiterRouter(t, cfg)

		for i := 0; i < 200; i++ {
			ip := fmt.Sprintf("10.%d.%d.%d:1", i/65536, (i/256)%256, i%256)
			assert.Equal(t, http.StatusOK, limiterRequest(r, ip, "", "").Code)
		}
		assert.LessOrEqual(t, rl.Size(), 5, "the map must never exceed the hard cap")
	})

	t.Run("ttl sweep drops idle keys", func(t *testing.T) {
		cfg := limiterBaseConfig()
		cfg.IPRate = rate.Limit(1_000_000)
		cfg.IPBurst = 1_000_000
		cfg.IdentifierKeys = nil
		cfg.MaxEntries = 1000
		cfg.TTL = 20 * time.Millisecond
		cfg.SweepInterval = 20 * time.Millisecond
		r, rl := newLimiterRouter(t, cfg)

		for i := 0; i < 50; i++ {
			ip := fmt.Sprintf("10.0.0.%d:1", i)
			assert.Equal(t, http.StatusOK, limiterRequest(r, ip, "", "").Code)
		}
		assert.Equal(t, 50, rl.Size())

		time.Sleep(50 * time.Millisecond)
		// A request after the sweep interval triggers the inline eviction.
		assert.Equal(t, http.StatusOK, limiterRequest(r, "10.9.9.9:1", "", "").Code)
		assert.LessOrEqual(t, rl.Size(), 2, "idle keys must be evicted by the TTL sweep")
	})
}

// TestRateLimit_AuthRoutesWired — the /auth-accounts routes and
// POST /auth/register-personnel actually carry the limiter, configured from the
// RATE_LIMIT_* environment.
func TestRateLimit_AuthRoutesWired(t *testing.T) {
	t.Setenv("RATE_LIMIT_IP_PER_MIN", "1")
	t.Setenv("RATE_LIMIT_IP_BURST", "2")
	t.Setenv("RATE_LIMIT_IDENTIFIER_PER_MIN", "1000")
	t.Setenv("RATE_LIMIT_IDENTIFIER_BURST", "1000")

	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	// Empty-body requests answer 400, but each still consumes one per-IP token.
	for i := 0; i < 2; i++ {
		w := do(r, http.MethodPost, "/api/auth-accounts/login", "", "")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}
	assert429(t, do(r, http.MethodPost, "/api/auth-accounts/login", "", ""))

	// register-personnel shares the SAME limiter, so the budget is exhausted
	// across routes, not per route.
	assert429(t, do(r, http.MethodPost, "/api/auth/register-personnel", "", `{}`))
}

// TestRateLimit_LegitLoginUnaffected — with the default (unset) config a normal
// register+login still succeeds well under the limit.
func TestRateLimit_LegitLoginUnaffected(t *testing.T) {
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	registerClient(t, r) // 201
	loginBody := `{"identifier":"ana.lopez@test.com","password":"password123","user_type":"client"}`
	w := do(r, http.MethodPost, "/api/auth-accounts/login", "", loginBody)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
