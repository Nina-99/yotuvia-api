package middleware

import (
	"net/http"
	"os"
	"strconv"

	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// DiskGuard refuses an upload when the volume that holds the uploads is running
// out of room.
//
// A per-file cap (5 MB receipts, 100 MB videos) bounds ONE request, not the sum:
// at the global 300 req/min per IP, receipts alone can write 1.5 GB per minute,
// and rotating IPs has no ceiling. `minFreeBytes` is the floor that protects the
// whole machine — on a single-volume VPS a full disk takes Postgres with it, so
// running out of space is an outage rather than a failed upload.
//
// It answers 503, not 400: the request is well-formed and the caller did nothing
// wrong; the server simply cannot accept writes right now.
func DiskGuard(minFreeBytes int64) gin.HandlerFunc {
	dir := utils.UploadsRoot()

	return func(ctx *gin.Context) {
		if minFreeBytes <= 0 {
			ctx.Next()
			return
		}
		free, err := utils.FreeBytes(dir)
		if err != nil {
			// Cannot stat the volume: never block uploads on a probe failure.
			// The alternative (fail closed) turns a transient stat error into a
			// full upload outage.
			ctx.Next()
			return
		}
		if free < uint64(minFreeBytes) {
			if utils.Log != nil {
				utils.Log.Errorf("[DISK] refusing upload: %d bytes free on %s (floor %d)",
					free, dir, minFreeBytes)
			}
			ctx.AbortWithStatusJSON(http.StatusServiceUnavailable,
				gin.H{"error": "not enough storage available, try again later"})
			return
		}
		ctx.Next()
	}
}

// DiskGuardFromEnv resolves MIN_FREE_DISK_BYTES, defaulting to 2 GiB.
func DiskGuardFromEnv() gin.HandlerFunc {
	return DiskGuard(minFreeDiskBytes())
}

func minFreeDiskBytes() int64 {
	const defaultFloor = 2 << 30 // 2 GiB
	raw := os.Getenv("MIN_FREE_DISK_BYTES")
	if raw == "" {
		return defaultFloor
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return defaultFloor
	}
	return n
}

// UploadRateLimitConfig is deliberately much tighter than the global limiter: a
// human uploads a handful of photos, never dozens per minute. This is the cheap
// control that keeps a single IP from writing gigabytes before the disk guard
// has anything to say.
func UploadRateLimitConfig() RateLimitConfig {
	cfg := DefaultRateLimitConfig()
	cfg.IPRate = rate.Limit(12.0 / 60.0) // 12 per minute
	cfg.IPBurst = 6
	cfg.IdentifierKeys = nil

	if v, ok := envFloat("RATE_LIMIT_UPLOAD_PER_MIN"); ok && v > 0 {
		cfg.IPRate = rate.Limit(v / 60.0)
	}
	if v, ok := envInt("RATE_LIMIT_UPLOAD_BURST"); ok && v > 0 {
		cfg.IPBurst = v
	}
	return cfg
}

// UploadGuard is the pair every upload route needs: a tight per-IP rate limit
// and a free-space floor checked before the handler touches the disk.
func UploadGuard() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		RateLimit(UploadRateLimitConfig()),
		DiskGuardFromEnv(),
	}
}

// WithUploadGuard prepends the upload guard chain to the given handlers, so a
// route reads as: POST("/upload", middleware.WithUploadGuard(ctrl.Upload)...)
//
// Go only expands a slice in the final argument position, which is why this
// helper exists instead of inlining `UploadGuard()..., handler`.
func WithUploadGuard(handlers ...gin.HandlerFunc) []gin.HandlerFunc {
	return append(UploadGuard(), handlers...)
}
