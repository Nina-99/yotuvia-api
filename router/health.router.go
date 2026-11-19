package router

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// healthPingTimeout bounds the database probe so a hung connection cannot turn
// the readiness endpoint into a hung request of its own.
const healthPingTimeout = 2 * time.Second

// HealthRoutes mounts the readiness probe used by the deploy script, systemd
// and any uptime monitor.
//
// It answers 200 only when the process can reach the database: a process that
// serves HTTP while Postgres is gone is not ready, and reporting it as healthy
// turns every deploy verification and every alert into a lie.
//
// It is mounted directly on the engine (not under the /api group) so it skips
// the global rate limiter and body limit: a fixed-size response must not be
// throttled, otherwise the probe fails exactly when traffic is high.
//
// The body is deliberately fixed: no error strings, hostnames or versions, all
// of which would be free reconnaissance for an unauthenticated caller.
func HealthRoutes(r *gin.Engine, db *gorm.DB) {
	r.GET("/api/healthz", func(ctx *gin.Context) {
		if db == nil {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unavailable",
				"db":     "unconfigured",
			})
			return
		}

		sqlDB, err := db.DB()
		if err != nil {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unavailable",
				"db":     "error",
			})
			return
		}

		pingCtx, cancel := context.WithTimeout(ctx.Request.Context(), healthPingTimeout)
		defer cancel()

		if err := sqlDB.PingContext(pingCtx); err != nil {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unavailable",
				"db":     "down",
			})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"db":     "ok",
		})
	})
}
