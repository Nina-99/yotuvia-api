package tests

import (
	"net/http"
	"testing"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/router"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// fullWiringRouter mounts EVERY router exactly as router.go does, over the same
// SQLite schema. Recovery turns a handler panic into a 500 so the no-5xx guard
// can assert on it instead of crashing the test binary.
func fullWiringRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	validate := validator.New()

	r := gin.New()
	r.Use(gin.Recovery())
	api := r.Group("/api")
	{
		router.AuthAccountRoutes(api, db, validate, utils.NewFakeMailer())
		router.AvailabilityRoutes(api, db, validate)
		router.BeautyCenterRoutes(api, db, validate)
		router.BeautyCenterImageRoutes(api, db, validate)
		router.CategoryRoutes(api, db, validate)
		router.ClientRoutes(api, db, validate)
		router.DashboardRoutes(api, db, validate)
		router.FavoriteRoutes(api, db, validate)
		router.FeedPostRoutes(api, db, validate)
		router.VideoCommentRoutes(api, db, validate)
		router.LoyaltyRoutes(api, db, validate)
		router.TokenRoutes(api, db, validate)
		router.TreatmentRoutes(api, db, validate)
		router.PaymentConfigRoutes(api, db, validate)
		router.PersonnelRoutes(api, db, validate)
		router.PersonnelInvitationRoutes(api, db, validate)
		router.PersonnelImageRoutes(api, db, validate)
		router.PersonnelServiceRoutes(api, db, validate)
		router.PostLikeRoutes(api, db, validate)
		router.PromotionRoutes(api, db, validate)
		router.ReservationRoutes(api, db, validate)
		router.ReviewRoutes(api, db, validate)
		router.SavedVideoRoutes(api, db, validate)
		router.NotificationRoutes(api, db)
		router.ServiceRoutes(api, db, validate)
		router.SocialMediaRoutes(api, db, validate)
		router.VideoWatchEventRoutes(api, db, validate)
	}
	router.UploadRoutes(api)

	// Seed one known row per entity a path param might hit, so an id like 1
	// resolves to a real resource where possible. A 404 still counts as a pass;
	// these rows exist so mutating routes exercise their wiring on a real row
	// instead of tripping a missing-row 500 in an unrelated controller.
	seedDashboardCenter(t, db, 1, "Wiring Center", "wiring@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedClient(t, db, 1, "wiring-client", "wiring-client@test.com")
	seedDashboardService(t, db, 1, 1, "Wiring Cut", true)
	if err := db.Create(&models.Category{ID: 1, BeautyCenterID: 1, Name: "Wiring Cat"}).Error; err != nil {
		t.Fatalf("failed to seed category: %v", err)
	}
	if err := db.Create(&models.Review{Model: gorm.Model{ID: 1}, BeautyCenterID: 1, ClientID: 1, Rating: 5, Comment: "wiring"}).Error; err != nil {
		t.Fatalf("failed to seed review: %v", err)
	}
	if err := db.Create(&models.SocialMedia{Model: gorm.Model{ID: 1}, BeautyCenterID: 1}).Error; err != nil {
		t.Fatalf("failed to seed social media: %v", err)
	}

	return r, db
}

// wiringRoute is one route from the mounted composition root plus a role whose
// token is ALLOWED to reach the handler.
type wiringRoute struct {
	name   string
	method string
	path   string
	role   string
}

// TestRouterWiring_No5xxWithValidToken is the strongest structural guard in the
// suite: with a token for an allowed role, NO route may answer 5xx.
//
// Acceptable outcomes are 2xx (worked), 400 (bad/empty body), 403 (service-level
// ownership), 404 (missing row/param), 409 (state conflict) and 413. A 500 or a
// panic is always a wiring failure — this catches both a nil dependency on a
// reachable path (the original /feed-posts/personalized bug) and a wrong-
// POSITION argument wiring where the types match but two arguments are swapped.
func TestRouterWiring_No5xxWithValidToken(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, _ := fullWiringRouter(t)

	routes := []wiringRoute{
		// ── auth ──
		{"auth login", http.MethodPost, "/api/auth-accounts/login", "client"},
		{"auth social", http.MethodPost, "/api/auth-accounts/social", "client"},
		{"auth register", http.MethodPost, "/api/auth-accounts/register", "client"},
		{"auth refresh", http.MethodPost, "/api/auth-accounts/refresh", "client"},
		{"auth logout", http.MethodPost, "/api/auth-accounts/logout", "client"},
		{"auth register personnel", http.MethodPost, "/api/auth/register-personnel", "client"},
		// ── availability search ──
		{"availability search", http.MethodGet, "/api/availability/search", "client"},
		// ── beauty centers ──
		{"beauty center list", http.MethodGet, "/api/beauty-centers", "client"},
		{"beauty center by id", http.MethodGet, "/api/beauty-centers/1", "client"},
		{"beauty center available slots", http.MethodGet, "/api/beauty-centers/1/available-slots", "client"},
		{"beauty center search", http.MethodGet, "/api/beauty-centers/search", "client"},
		{"beauty center top rated", http.MethodGet, "/api/beauty-centers/top-rated", "client"},
		{"beauty center nearby", http.MethodGet, "/api/beauty-centers/nearby", "client"},
		// NOT COVERED: GET /api/beauty-centers/ranking runs a Postgres LOG()
		// aggregate (ranking score) that SQLite cannot evaluate, so it 500s on
		// the dialect itself — a fixture limitation, not a wiring bug. It is
		// verified against Postgres in production, not here.
		{"beauty center update", http.MethodPut, "/api/beauty-centers/1", "beauty_center"},
		{"beauty center delete", http.MethodDelete, "/api/beauty-centers/1", "beauty_center"},
		// ── beauty center images ──
		{"beauty image upload", http.MethodPost, "/api/beauty-image/upload", "client"},
		{"beauty image create", http.MethodPost, "/api/beauty-image", "client"},
		{"beauty image reorder", http.MethodPatch, "/api/beauty-image/reorder", "client"},
		{"beauty image update", http.MethodPatch, "/api/beauty-image/1", "client"},
		{"beauty image by id", http.MethodGet, "/api/beauty-image/1", "client"},
		{"beauty image delete", http.MethodDelete, "/api/beauty-image/1", "client"},
		{"beauty image by center", http.MethodGet, "/api/beauty-image/beauty_center/1", "client"},
		// ── categories ──
		{"category create", http.MethodPost, "/api/category", "client"},
		{"category update", http.MethodPut, "/api/category/1", "client"},
		{"category delete", http.MethodDelete, "/api/category/1", "client"},
		{"category templates", http.MethodGet, "/api/category/templates", "client"},
		{"category list", http.MethodGet, "/api/category", "client"},
		{"category by id", http.MethodGet, "/api/category/1", "client"},
		// ── client ──
		{"client find all", http.MethodGet, "/api/client", "beauty_center"},
		{"client by id", http.MethodGet, "/api/client/1", "client"},
		{"client me", http.MethodGet, "/api/client/me", "client"},
		{"client delete", http.MethodDelete, "/api/client/1", "client"},
		{"client update", http.MethodPut, "/api/client/1", "client"},
		{"client avatar", http.MethodPost, "/api/client/avatar", "client"},
		// ── dashboard ──
		{"dashboard stats", http.MethodGet, "/api/dashboard/stats", "beauty_center"},
		// ── favorites ──
		{"favorite create", http.MethodPost, "/api/favorites", "client"},
		{"favorite delete", http.MethodDelete, "/api/favorites/1", "client"},
		{"favorites by client", http.MethodGet, "/api/clients/1/favorites", "client"},
		// ── feed ──
		{"feed public list", http.MethodGet, "/api/feed-posts", "client"},
		{"feed by center", http.MethodGet, "/api/feed-posts/center/1", "client"},
		{"feed by personnel", http.MethodGet, "/api/feed-posts/personnel/1", "client"},
		{"feed increment views", http.MethodPost, "/api/feed-posts/1/views", "client"},
		{"feed increment book taps", http.MethodPost, "/api/feed-posts/1/book-taps", "client"},
		{"feed create", http.MethodPost, "/api/feed-posts", "personnel"},
		{"feed update", http.MethodPut, "/api/feed-posts/1", "client"},
		{"feed delete", http.MethodDelete, "/api/feed-posts/1", "client"},
		{"feed mine", http.MethodGet, "/api/feed-posts/mine", "client"},
		{"feed suspend", http.MethodPut, "/api/feed-posts/1/suspend", "beauty_center"},
		{"feed personalized", http.MethodGet, "/api/feed-posts/personalized", "client"},
		// ── video comments ──
		{"comment create", http.MethodPost, "/api/feed-posts/1/comments", "client"},
		{"comment list", http.MethodGet, "/api/feed-posts/1/comments", "client"},
		{"comment replies", http.MethodGet, "/api/feed-posts/1/comments/1", "client"},
		{"comment reply", http.MethodPost, "/api/feed-posts/1/comments/1/reply", "client"},
		{"comment update", http.MethodPatch, "/api/feed-posts/1/comments/1", "client"},
		{"comment delete", http.MethodDelete, "/api/feed-posts/1/comments/1", "client"},
		{"comment hide", http.MethodPatch, "/api/feed-posts/1/comments/1/hide", "client"},
		// ── loyalty ──
		{"loyalty create program", http.MethodPost, "/api/loyalty", "beauty_center"},
		{"loyalty client cards", http.MethodGet, "/api/loyalty/client/1", "client"},
		{"loyalty by center", http.MethodGet, "/api/loyalty/center/1", "beauty_center"},
		{"loyalty settings", http.MethodGet, "/api/loyalty/center/1/settings", "beauty_center"},
		{"loyalty settings update", http.MethodPut, "/api/loyalty/center/1/settings", "beauty_center"},
		{"loyalty update program", http.MethodPut, "/api/loyalty/1", "beauty_center"},
		{"loyalty delete program", http.MethodDelete, "/api/loyalty/1", "beauty_center"},
		{"loyalty toggle program", http.MethodPatch, "/api/loyalty/1/toggle", "beauty_center"},
		// ── tokens ──
		{"tokens balance", http.MethodGet, "/api/tokens/balance", "beauty_center"},
		{"tokens history", http.MethodGet, "/api/tokens/history", "beauty_center"},
		{"tokens topup", http.MethodPost, "/api/tokens/topup", "beauty_center"},
		{"tokens topups", http.MethodGet, "/api/tokens/topups", "beauty_center"},
		{"tokens missions", http.MethodGet, "/api/tokens/missions", "beauty_center"},
		{"tokens mission claim", http.MethodPost, "/api/tokens/missions/1/claim", "beauty_center"},
		{"tokens referral info", http.MethodGet, "/api/tokens/referral-info", "beauty_center"},
		{"tokens referrals", http.MethodGet, "/api/tokens/referrals", "beauty_center"},
		{"tokens create referral", http.MethodPost, "/api/tokens/referrals", "beauty_center"},
		{"admin topups list", http.MethodGet, "/api/admin/tokens/topups", "beauty_center"},
		{"admin topup process", http.MethodPut, "/api/admin/tokens/topups/1", "beauty_center"},
		// ── treatments (public vocabulary) ──
		{"treatment catalog", http.MethodGet, "/api/treatments", "client"},
		// ── payment config ──
		{"payment config get", http.MethodGet, "/api/config/payment", "client"},
		{"tier config get", http.MethodGet, "/api/config/tiers", "client"},
		{"payment config update", http.MethodPut, "/api/admin/config/payment", "beauty_center"},
		{"payment qr upload", http.MethodPost, "/api/admin/config/payment/qr", "beauty_center"},
		// ── personnel ──
		{"personnel find all", http.MethodGet, "/api/personnel", "client"},
		{"personnel find by id", http.MethodGet, "/api/personnel/1", "client"},
		{"personnel update", http.MethodPut, "/api/personnel/1", "beauty_center"},
		{"personnel delete", http.MethodDelete, "/api/personnel/1", "beauty_center"},
		{"personnel update me", http.MethodPatch, "/api/personnel/me", "personnel"},
		{"personnel follow", http.MethodPost, "/api/personnel/1/follow", "client"},
		{"personnel unfollow", http.MethodDelete, "/api/personnel/1/follow", "client"},
		// ── personnel invitations ──
		{"invite by token", http.MethodGet, "/api/personnel/invite/abc", "client"},
		{"invite accept", http.MethodPost, "/api/personnel/accept-invite", "client"},
		{"invitation generate", http.MethodPost, "/api/personnel/invitations", "beauty_center"},
		{"invitation list", http.MethodGet, "/api/personnel/invitations", "beauty_center"},
		// ── personnel images ──
		{"personnel image upload", http.MethodPost, "/api/personnel-images/upload", "client"},
		{"personnel image create", http.MethodPost, "/api/personnel-images", "client"},
		{"personnel image by id", http.MethodGet, "/api/personnel-images/1", "client"},
		{"personnel image delete", http.MethodDelete, "/api/personnel-images/1", "client"},
		{"personnel image by personnel", http.MethodGet, "/api/personnel-images/personnel/1", "client"},
		// ── personnel services ──
		{"personnel service create", http.MethodPost, "/api/personnel-services", "beauty_center"},
		{"personnel service delete", http.MethodDelete, "/api/personnel-services/1", "beauty_center"},
		{"personnel service list", http.MethodGet, "/api/personnel-services", "client"},
		// ── post likes ──
		{"post likes by post", http.MethodGet, "/api/post-likes/post/1", "client"},
		{"post like create", http.MethodPost, "/api/post-likes", "client"},
		{"post like toggle", http.MethodPost, "/api/post-likes/toggle", "client"},
		{"post like delete", http.MethodDelete, "/api/post-likes/post/1", "client"},
		{"post like check", http.MethodGet, "/api/post-likes/post/1/check", "client"},
		// ── promotions ──
		{"promotion applicable", http.MethodGet, "/api/promotions/center/1/applicable", "client"},
		{"promotion create", http.MethodPost, "/api/promotions", "beauty_center"},
		{"promotion by center", http.MethodGet, "/api/promotions/center/1", "beauty_center"},
		{"promotion by id", http.MethodGet, "/api/promotions/1", "beauty_center"},
		{"promotion update", http.MethodPut, "/api/promotions/1", "beauty_center"},
		{"promotion delete", http.MethodDelete, "/api/promotions/1", "beauty_center"},
		// ── reservations ──
		{"reservation create", http.MethodPost, "/api/reservation/", "client"},
		{"reservation admin create", http.MethodPost, "/api/reservation/admin", "beauty_center"},
		{"reservation find all", http.MethodGet, "/api/reservation", "beauty_center"},
		{"reservation by client", http.MethodGet, "/api/reservation/client/1", "client"},
		{"reservation upcoming", http.MethodGet, "/api/reservation/client/1/upcoming", "client"},
		{"reservation by center", http.MethodGet, "/api/reservation/center/1", "beauty_center"},
		{"reservation by id", http.MethodGet, "/api/reservation/1", "client"},
		{"reservation update", http.MethodPut, "/api/reservation/1", "client"},
		{"reservation delete", http.MethodDelete, "/api/reservation/1", "client"},
		{"reservation status", http.MethodPatch, "/api/reservation/1/status", "beauty_center"},
		{"reservation assign", http.MethodPatch, "/api/reservation/1/assign", "beauty_center"},
		{"reservation cancel", http.MethodPatch, "/api/reservation/1/cancel", "client"},
		{"reservation upload proof", http.MethodPost, "/api/reservation/1/proof", "client"},
		{"reservation review proof", http.MethodPatch, "/api/reservation/1/proof", "beauty_center"},
		{"reservation generate qr", http.MethodPost, "/api/reservation/generate-qr/1", "client"},
		{"reservation verify qr", http.MethodGet, "/api/reservation/verify-qr", "beauty_center"},
		{"reservation my agenda", http.MethodGet, "/api/reservation/me", "personnel"},
		{"reservation my metrics", http.MethodGet, "/api/reservation/me/metrics", "personnel"},
		{"reservation my status", http.MethodPatch, "/api/reservation/me/1/status", "personnel"},
		{"waitlist join", http.MethodPost, "/api/reservation/waitlist", "client"},
		{"waitlist mine", http.MethodGet, "/api/reservation/waitlist/mine", "client"},
		{"waitlist leave", http.MethodDelete, "/api/reservation/waitlist/1", "client"},
		{"waitlist claim", http.MethodPost, "/api/reservation/waitlist/1/claim", "client"},
		// ── reviews ──
		{"review by id", http.MethodGet, "/api/reviews/1", "client"},
		{"review by center", http.MethodGet, "/api/reviews/beauty-center/1", "client"},
		{"review by client", http.MethodGet, "/api/reviews/client/1", "client"},
		{"review create", http.MethodPost, "/api/reviews/", "client"},
		{"review update", http.MethodPut, "/api/reviews/1", "client"},
		{"review delete", http.MethodDelete, "/api/reviews/1", "client"},
		{"review can review", http.MethodGet, "/api/reviews/can-review/1", "client"},
		// ── saved videos ──
		{"saved toggle", http.MethodPost, "/api/saved-videos/toggle", "client"},
		{"saved check", http.MethodGet, "/api/saved-videos/post/1/check", "client"},
		{"saved list", http.MethodGet, "/api/saved-videos", "client"},
		// ── services ──
		{"service create", http.MethodPost, "/api/services", "client"},
		{"service update", http.MethodPut, "/api/services/1", "client"},
		{"service delete", http.MethodDelete, "/api/services/1", "client"},
		{"service list", http.MethodGet, "/api/services", "client"},
		{"service templates", http.MethodGet, "/api/services/templates", "client"},
		{"service by id", http.MethodGet, "/api/services/1", "client"},
		// ── social media ──
		{"social media by center", http.MethodGet, "/api/beauty-centers/1/social-media", "client"},
		{"social media create", http.MethodPost, "/api/social-media", "client"},
		{"social media by id", http.MethodGet, "/api/social-media/1", "client"},
		{"social media update", http.MethodPut, "/api/social-media/1", "client"},
		{"social media delete", http.MethodDelete, "/api/social-media/1", "client"},
		// ── notifications ──
		{"notifications by client", http.MethodGet, "/api/notifications/client/1", "client"},
		{"notifications unread", http.MethodGet, "/api/notifications/client/1/unread", "client"},
		{"notifications count", http.MethodGet, "/api/notifications/client/1/count", "client"},
		{"notification read", http.MethodPatch, "/api/notifications/1/read", "client"},
		{"notifications read all", http.MethodPost, "/api/notifications/client/1/read-all", "client"},
		{"notifications personnel", http.MethodGet, "/api/notifications/personnel", "personnel"},
		{"push device register", http.MethodPost, "/api/push-devices/register", "client"},
		{"notifications center list", http.MethodGet, "/api/notifications/center", "beauty_center"},
		{"notifications center unread", http.MethodGet, "/api/notifications/center/unread", "beauty_center"},
		{"notifications center count", http.MethodGet, "/api/notifications/center/count", "beauty_center"},
		{"notification center read", http.MethodPatch, "/api/notifications/center/1/read", "beauty_center"},
		// ── video watch events ──
		{"watch record", http.MethodPost, "/api/video-watch-events", "client"},
		// ── uploads ──
		{"upload receipt", http.MethodPost, "/api/upload/receipt", "client"},
	}

	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			token := roleToken(t, 1, 1, rt.role)
			w := do(r, rt.method, rt.path, token, "")
			assert.Less(t, w.Code, http.StatusInternalServerError,
				"route %s %s must not 5xx with a valid %s token (got %d: %s)",
				rt.method, rt.path, rt.role, w.Code, w.Body.String())
		})
	}
}
