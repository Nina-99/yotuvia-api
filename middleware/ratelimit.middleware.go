package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimitConfig configures a two-dimensional request limiter.
//
// Two dimensions are enforced because they stop different attacks:
//   - Per-IP stops a single host sweeping many accounts.
//   - Per-identifier (the email/username being attempted) stops a distributed
//     attack aimed at ONE account from many IPs. Neither dimension alone closes
//     both.
type RateLimitConfig struct {
	// IPRate/IPBurst are the token-bucket refill rate (events/second) and the
	// initial burst for the per-IP dimension.
	IPRate  rate.Limit
	IPBurst int
	// IdentifierRate/IdentifierBurst govern the per-identifier dimension.
	IdentifierRate  rate.Limit
	IdentifierBurst int

	// TTL is how long an idle key is kept before an eviction sweep drops it.
	// SweepInterval is the minimum gap between opportunistic sweeps (a sweep
	// runs inline on the next request once this much time has elapsed).
	TTL           time.Duration
	SweepInterval time.Duration
	// MaxEntries is the hard ceiling per dimension. Even under a distributed
	// scan that mints unlimited fresh keys, the maps can never grow past this,
	// so the limiter itself cannot be turned into an unbounded-memory DoS.
	MaxEntries int

	// IdentifierKeys are the JSON body fields probed, in order, for the
	// identifier to throttle. The first non-empty string wins.
	IdentifierKeys []string
	// MaxBodyBytes caps how much of a request body is inspected. Larger bodies
	// are left untouched and simply skip the per-identifier dimension.
	MaxBodyBytes int64
}

// DefaultRateLimitConfig returns production-sane defaults: 30 requests/min per
// IP (burst 20) and 5 attempts/min per identifier (burst 5), with a 10 minute
// idle TTL, a 1 minute sweep interval and a 10000-key hard cap per dimension.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		IPRate:          rate.Limit(30.0 / 60.0),
		IPBurst:         20,
		IdentifierRate:  rate.Limit(5.0 / 60.0),
		IdentifierBurst: 5,
		TTL:             10 * time.Minute,
		SweepInterval:   time.Minute,
		MaxEntries:      10000,
		IdentifierKeys:  []string{"identifier", "email", "username"},
		MaxBodyBytes:    1 << 20, // 1 MiB
	}
}

// RateLimitConfigFromEnv overlays RATE_LIMIT_* environment variables on the
// defaults, following the plain os.Getenv-with-fallback style of
// configs/scheduler.go. A malformed or non-positive value is ignored, so the
// defaults always produce a working limiter.
func RateLimitConfigFromEnv() RateLimitConfig {
	cfg := DefaultRateLimitConfig()
	if v, ok := envFloat("RATE_LIMIT_IP_PER_MIN"); ok && v > 0 {
		cfg.IPRate = rate.Limit(v / 60.0)
	}
	if v, ok := envInt("RATE_LIMIT_IP_BURST"); ok && v > 0 {
		cfg.IPBurst = v
	}
	if v, ok := envFloat("RATE_LIMIT_IDENTIFIER_PER_MIN"); ok && v > 0 {
		cfg.IdentifierRate = rate.Limit(v / 60.0)
	}
	if v, ok := envInt("RATE_LIMIT_IDENTIFIER_BURST"); ok && v > 0 {
		cfg.IdentifierBurst = v
	}
	if v, ok := envInt("RATE_LIMIT_TTL_SEC"); ok && v > 0 {
		cfg.TTL = time.Duration(v) * time.Second
	}
	if v, ok := envInt("RATE_LIMIT_SWEEP_SEC"); ok && v > 0 {
		cfg.SweepInterval = time.Duration(v) * time.Second
	}
	if v, ok := envInt("RATE_LIMIT_MAX_ENTRIES"); ok && v > 0 {
		cfg.MaxEntries = v
	}
	return cfg
}

func envInt(name string) (int, bool) {
	raw := os.Getenv(name)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return n, true
}

func envFloat(name string) (float64, bool) {
	raw := os.Getenv(name)
	if raw == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// normalizeRateLimitConfig fills zero-valued fields from the defaults so a
// partially specified config never yields a zero burst (which would block every
// request) or an unbounded map.
func normalizeRateLimitConfig(cfg RateLimitConfig) RateLimitConfig {
	def := DefaultRateLimitConfig()
	if cfg.IPRate <= 0 {
		cfg.IPRate = def.IPRate
	}
	if cfg.IPBurst <= 0 {
		cfg.IPBurst = def.IPBurst
	}
	if cfg.IdentifierRate <= 0 {
		cfg.IdentifierRate = def.IdentifierRate
	}
	if cfg.IdentifierBurst <= 0 {
		cfg.IdentifierBurst = def.IdentifierBurst
	}
	if cfg.TTL <= 0 {
		cfg.TTL = def.TTL
	}
	if cfg.SweepInterval <= 0 {
		cfg.SweepInterval = def.SweepInterval
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = def.MaxEntries
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = def.MaxBodyBytes
	}
	return cfg
}

type rateEntry struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// RateLimiter holds the per-IP and per-identifier token buckets. It is safe for
// concurrent use.
type RateLimiter struct {
	cfg RateLimitConfig

	mu        sync.Mutex
	ips       map[string]*rateEntry
	ids       map[string]*rateEntry
	lastSweep time.Time
}

// NewRateLimiter builds a limiter from cfg. Zero-valued fields fall back to the
// production defaults.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	cfg = normalizeRateLimitConfig(cfg)
	return &RateLimiter{
		cfg:       cfg,
		ips:       make(map[string]*rateEntry),
		ids:       make(map[string]*rateEntry),
		lastSweep: time.Now(),
	}
}

// RateLimit is the reusable middleware factory: other sensitive endpoints can
// opt in with their own config without touching the auth wiring.
func RateLimit(cfg RateLimitConfig) gin.HandlerFunc {
	return NewRateLimiter(cfg).Handler()
}

// Size reports the total tracked keys across both dimensions. Exposed so tests
// (and operators) can assert the maps stay bounded.
func (rl *RateLimiter) Size() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.ips) + len(rl.ids)
}

// Handler enforces the per-IP dimension first (cheap, protects CPU-heavy
// handlers like bcrypt), then the per-identifier dimension, which requires
// reading the JSON body. The body is ALWAYS restored before the handler runs.
func (rl *RateLimiter) Handler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		now := time.Now()

		rl.mu.Lock()
		if now.Sub(rl.lastSweep) >= rl.cfg.SweepInterval {
			rl.sweepLocked(now)
		}
		ipKey := ctx.ClientIP()
		ipAllowed := rl.allowLocked(rl.ips, ipKey, rl.cfg.IPRate, rl.cfg.IPBurst, now)
		rl.mu.Unlock()

		if !ipAllowed {
			rl.reject(ctx, "ip", ipKey)
			return
		}

		// The identifier lives in the body, which Gin has not bound yet. We read
		// it here (bounded) and restore it untouched, so the identification
		// happens once, uniformly for every opted-in endpoint, while controllers
		// keep their existing ShouldBindJSON contract.
		idKey := ""
		if len(rl.cfg.IdentifierKeys) > 0 {
			idKey = extractIdentifier(ctx, rl.cfg.IdentifierKeys, rl.cfg.MaxBodyBytes)
		}
		if idKey != "" {
			rl.mu.Lock()
			idAllowed := rl.allowLocked(rl.ids, idKey, rl.cfg.IdentifierRate, rl.cfg.IdentifierBurst, now)
			rl.mu.Unlock()
			if !idAllowed {
				rl.reject(ctx, "identifier", idKey)
				return
			}
		}

		ctx.Next()
	}
}

// allowLocked returns whether key may proceed and records the hit. The caller
// must hold rl.mu. When a dimension is at its hard cap, expired keys are swept
// and, only if still full, the longest-idle key is dropped to make room — the
// map is bounded no matter how many distinct keys arrive.
func (rl *RateLimiter) allowLocked(m map[string]*rateEntry, key string, limit rate.Limit, burst int, now time.Time) bool {
	e, ok := m[key]
	if !ok {
		if len(m) >= rl.cfg.MaxEntries {
			rl.evictLocked(m, now)
		}
		e = &rateEntry{lim: rate.NewLimiter(limit, burst)}
		m[key] = e
	}
	e.lastSeen = now
	return e.lim.AllowN(now, 1)
}

// sweepLocked drops every key idle for longer than the TTL. Runs inline, at
// most once per SweepInterval, so no background goroutine (and no goroutine
// leak across short-lived test routers) is needed.
func (rl *RateLimiter) sweepLocked(now time.Time) {
	rl.lastSweep = now
	cutoff := now.Add(-rl.cfg.TTL)
	for k, e := range rl.ips {
		if e.lastSeen.Before(cutoff) {
			delete(rl.ips, k)
		}
	}
	for k, e := range rl.ids {
		if e.lastSeen.Before(cutoff) {
			delete(rl.ids, k)
		}
	}
}

// evictLocked first drops TTL-expired keys, then evicts longest-idle keys until
// there is room for one more. The caller must hold rl.mu.
func (rl *RateLimiter) evictLocked(m map[string]*rateEntry, now time.Time) {
	cutoff := now.Add(-rl.cfg.TTL)
	for k, e := range m {
		if e.lastSeen.Before(cutoff) {
			delete(m, k)
		}
	}
	for len(m) >= rl.cfg.MaxEntries {
		var oldestKey string
		var oldest time.Time
		first := true
		for k, e := range m {
			if first || e.lastSeen.Before(oldest) {
				oldestKey, oldest, first = k, e.lastSeen, false
			}
		}
		if first {
			break
		}
		delete(m, oldestKey)
	}
}

func (rl *RateLimiter) reject(ctx *gin.Context, dimension, key string) {
	if utils.Log != nil {
		// The key is an email/username or an IP — never a password or token.
		// Logging the identifier is intentional: it lets an operator trace a
		// targeted attack to the account being hammered.
		utils.Log.Warnf("[RATE LIMIT] throttled %s %s on %s dimension (key=%q)",
			ctx.Request.Method, ctx.Request.URL.Path, dimension, key)
	}
	ctx.Header("Retry-After", strconv.Itoa(rl.retryAfterSeconds(dimension)))
	ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
}

// retryAfterSeconds estimates how long until one token refills on the tripped
// dimension, rounded up to at least one second.
func (rl *RateLimiter) retryAfterSeconds(dimension string) int {
	limit := rl.cfg.IPRate
	if dimension == "identifier" {
		limit = rl.cfg.IdentifierRate
	}
	if limit <= 0 {
		return 1
	}
	secs := int(math.Ceil(1.0 / float64(limit)))
	if secs < 1 {
		secs = 1
	}
	return secs
}

// extractIdentifier reads the request body (bounded by maxBody), restores it on
// ctx.Request.Body and returns the first non-empty identifier found among keys.
// It returns "" for a missing body, a non-JSON body, an oversized body, or a
// body without any of the keys — in every case the body is left intact for the
// handler.
func extractIdentifier(ctx *gin.Context, keys []string, maxBody int64) string {
	if ctx.Request == nil || ctx.Request.Body == nil {
		return ""
	}

	original := ctx.Request.Body
	data, err := io.ReadAll(io.LimitReader(original, maxBody+1))
	// Restore everything read plus whatever is still unread. This holds even for
	// an oversized body, where only maxBody+1 bytes were consumed.
	ctx.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(data), original))
	if err != nil || int64(len(data)) > maxBody {
		return ""
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	for _, k := range keys {
		v, ok := raw[k]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			continue
		}
		if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
			return s
		}
	}
	return ""
}

// GlobalRateLimitConfigFromEnv builds the broad per-IP limiter that fronts the
// whole API. It is deliberately generous (300 req/min, burst 100): its job is
// to absorb a flood or a scraping sweep, not to police normal use. Auth routes
// keep their own, much tighter, shared limiter on top of this one.
//
// IdentifierKeys is empty: the global limiter must not read request bodies,
// both for cost and because it runs on every path (including large uploads).
func GlobalRateLimitConfigFromEnv() RateLimitConfig {
	cfg := DefaultRateLimitConfig()
	cfg.IPRate = rate.Limit(300.0 / 60.0)
	cfg.IPBurst = 100
	cfg.IdentifierKeys = nil

	if v, ok := envFloat("GLOBAL_RATE_LIMIT_IP_PER_MIN"); ok && v > 0 {
		cfg.IPRate = rate.Limit(v / 60.0)
	}
	if v, ok := envInt("GLOBAL_RATE_LIMIT_IP_BURST"); ok && v > 0 {
		cfg.IPBurst = v
	}
	return cfg
}

// PublicSearchRateLimitConfig bounds the expensive public reads (availability
// search, search/ranking previews).
//
// These endpoints are unauthenticated by design — a visitor must be able to see
// availability before signing up — but they are also the cheapest way to make
// the server do expensive work: one availability query walks a 14-day window
// across every eligible centre. A dedicated, tighter budget keeps the feature
// public without turning it into a free DoS amplifier or a scraping API.
func PublicSearchRateLimitConfig() RateLimitConfig {
	cfg := DefaultRateLimitConfig()
	cfg.IPRate = rate.Limit(30.0 / 60.0) // 30 per minute
	cfg.IPBurst = 15
	cfg.IdentifierKeys = nil

	if v, ok := envFloat("RATE_LIMIT_SEARCH_PER_MIN"); ok && v > 0 {
		cfg.IPRate = rate.Limit(v / 60.0)
	}
	if v, ok := envInt("RATE_LIMIT_SEARCH_BURST"); ok && v > 0 {
		cfg.IPBurst = v
	}
	return cfg
}
