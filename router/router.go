package router

import (
	"log"
	"os"
	"strings"

	"github.com/Nina-99/yottu-api/configs"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()
	ConfigureTrustedProxies(r)
	r.Use(middleware.GinLogger())

	// Serve uploaded files
	r.Static("/uploads", "./uploads")

	validate := validator.New()
	db := configs.DB

	api := r.Group("/api")
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
		PersonnelRoutes(api, db, validate)
		PersonnelInvitationRoutes(api, db, validate)
		PersonnelImageRoutes(api, db, validate)
		PersonnelServiceRoutes(api, db, validate)
		PostLikeRoutes(api, db, validate)
		PromotionRoutes(api, db, validate)
		ReservationRoutes(api, db, validate)
		ReviewRoutes(api, db, validate)
		SavedVideoRoutes(api, db, validate)
		NotificationRoutes(api, db)
		ServiceRoutes(api, db, validate)
		SocialMediaRoutes(api, db, validate)
		VideoWatchEventRoutes(api, db, validate)
		BoostRoutes(api, db, validate)
	}

	UploadRoutes(api)
	return r
}

func Run(addr string) {
	r := SetupRouter()
	log.Println("🚀 Server running on", addr)

	r.Run(addr)
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
