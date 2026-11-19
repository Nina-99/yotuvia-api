package router

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Nina-99/yotuvia-api/configs"
	"github.com/Nina-99/yotuvia-api/middleware"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()
	ConfigureTrustedProxies(r)
	r.Use(middleware.GinLogger())
	// CORS must run before any route so browser preflights are answered even
	// for unmatched OPTIONS requests (otherwise they fall through to 404).
	r.Use(middleware.CORS())
	// Security headers cover every response, including the static files served
	// from /uploads: the CSP makes any file a browser interprets as a document
	// inert, so a stored upload can never reach the web app's tokens.
	r.Use(middleware.SecurityHeaders())

	// Serve uploaded files
	r.Static("/uploads", "./uploads")

	validate := validator.New()
	db := configs.DB

	// Bind access tokens to their session row. Without this, a logged-out or
	// deactivated session kept working until its token expired (15 min); now the
	// revocation is visible on the very next request.
	middleware.SetSessionChecker(repository.NewUserSessionRepositoryImpl(db))

	api := r.Group("/api")
	// Broad per-IP flood guard in front of every API route. Auth keeps its own
	// tighter, shared limiter on top (see AuthAccountRoutes).
	api.Use(middleware.RateLimit(middleware.GlobalRateLimitConfigFromEnv()))
	// Cap non-multipart bodies so a JSON payload cannot be inflated into memory.
	api.Use(middleware.BodyLimit(bodyLimitFromEnv()))
	{
		AuthAccountRoutes(api, db, validate)
		AvailabilityRoutes(api, db, validate)
		BeautyCenterRoutes(api, db, validate)
		BeautyCenterImageRoutes(api, db, validate)
		CategoryRoutes(api, db, validate)
		ClientRoutes(api, db, validate)
		DashboardRoutes(api, db, validate)
		FavoriteRoutes(api, db, validate)
		FeedPostRoutes(api, db, validate)
		VideoCommentRoutes(api, db, validate)
		LoyaltyRoutes(api, db, validate)
		TokenRoutes(api, db, validate)
		TreatmentRoutes(api, db, validate)
		PaymentConfigRoutes(api, db, validate)
		PlatformRoutes(api, db, validate)
		PersonnelRoutes(api, db, validate)
		PersonnelInvitationRoutes(api, db, validate)
		PersonnelImageRoutes(api, db, validate)
		PersonnelServiceRoutes(api, db, validate)
		PostLikeRoutes(api, db, validate)
		PromotionRoutes(api, db, validate)
		ReservationRoutes(api, db, validate)
		TimeBlockRoutes(api, db, validate)
		ReviewRoutes(api, db, validate)
		SavedVideoRoutes(api, db, validate)
		NotificationRoutes(api, db)
		ServiceRoutes(api, db, validate)
		SocialMediaRoutes(api, db, validate)
		VideoWatchEventRoutes(api, db, validate)
		BoostRoutes(api, db, validate)
		TikTokRoutes(api, db, validate)
	}

	UploadRoutes(api)
	return r
}

// httpServerTimeouts are deliberately generous on the body/response side and
// tight on the two that matter for a denial-of-service:
//
//   - ReadHeaderTimeout kills Slowloris. `r.Run()` (http.ListenAndServe) sets NO
//     timeout at all, so a client that opens a connection and dribbles headers
//     holds it open forever; a few thousand of those sockets take the service
//     down for the price of 1 KB each.
//   - IdleTimeout reclaims keep-alive connections that stopped talking.
//
// Read/Write stay long because a legit 100 MB video upload or download over a
// mediocre mobile link can take minutes; shortening them would trade a
// theoretical slow-body attack for breaking real uploads. Everything below is
// still bounded, which is the point.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 10 * time.Minute
	writeTimeout      = 10 * time.Minute
	idleTimeout       = 60 * time.Second
)

// NewHTTPServer builds the production server. Exported so a test can assert the
// timeout configuration: a missing ReadHeaderTimeout is invisible at runtime
// until someone opens sockets and never finishes their headers.
func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    1 << 20,
	}
}

func Run(addr string) {
	srv := NewHTTPServer(addr, SetupRouter())
	log.Println("🚀 Server running on", addr)

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("🚀 Server stopped: %v", err)
	}
}

// ConfigureTrustedProxies sets Gin's trusted-proxy list from TRUSTED_PROXIES, a
// comma-separated list of the IPs/CIDRs of the reverse proxies in front of this
// service.
//
// Gin's DEFAULT trusts EVERY proxy, so ctx.ClientIP() would return whatever the
// client sends in X-Forwarded-For. A per-IP rate limit built on a
// client-controlled value is trivially bypassed by rotating a spoofed header,
// so the default here is to trust NOTHING: ClientIP() then falls back to the TCP
// RemoteAddr and a client-supplied X-Forwarded-For is ignored.
//
// OPERATIONAL REQUIREMENT: when deploying behind nginx / a load balancer, set
// TRUSTED_PROXIES to that proxy's IP/CIDR (e.g. "10.0.0.0/8,127.0.0.1/32").
// Otherwise every request is attributed to the proxy and shares one budget.
// An invalid value is rejected and falls back to trusting nothing, so a bad env
// var fails closed instead of silently trusting everyone.
func ConfigureTrustedProxies(r *gin.Engine) {
	raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES"))
	if raw != "" {
		var proxies []string
		for _, p := range strings.Split(raw, ",") {
			if p = strings.TrimSpace(p); p != "" {
				proxies = append(proxies, p)
			}
		}
		if len(proxies) > 0 {
			if err := r.SetTrustedProxies(proxies); err == nil {
				return
			} else if utils.Log != nil {
				utils.Log.Errorf("[PROXY] invalid TRUSTED_PROXIES %q: %v; trusting no proxy", raw, err)
			}
		}
	}
	// Unset, empty or invalid → trust nothing.
	_ = r.SetTrustedProxies(nil)
}

// bodyLimitFromEnv resolves the JSON/body cap. Defaults to 2 MiB: generous for
// any legitimate JSON payload this API accepts, small enough that a flood of
// oversized bodies cannot inflate the process. BODY_LIMIT_BYTES overrides it;
// a malformed or non-positive value falls back to the default rather than
// disabling the cap.
func bodyLimitFromEnv() int64 {
	const defaultBodyLimit = 2 << 20
	raw := strings.TrimSpace(os.Getenv("BODY_LIMIT_BYTES"))
	if raw == "" {
		return defaultBodyLimit
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return defaultBodyLimit
	}
	return n
}
