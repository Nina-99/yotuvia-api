package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

// AuthAccountRoutes mounts the auth surface. The optional mailer lets tests
// inject a recording fake; when omitted the process-wide env-configured mailer
// is used (SMTP when configured, otherwise the log-only dev fallback). Keeping
// it variadic preserves every existing call site.
func AuthAccountRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate, mailers ...utils.Mailer) {
	clientRepo := repository.NewClientRepositoryImpl(db)
	personnelRepo := repository.NewPersonnelRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	authRepo := repository.NewAuthAccountRepositoryImpl(db)
	sessionRepo := repository.NewUserSessionRepositoryImpl(db)
	tokenRepo := repository.NewAuthTokenRepositoryImpl(db)

	mailer := utils.GetMailer()
	if len(mailers) > 0 && mailers[0] != nil {
		mailer = mailers[0]
	}

	// Token service (for welcome bonus)
	tokenAccountRepo := repository.NewTokenAccountRepositoryImpl(db)
	tokenTopUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	tokenMissionRepo := repository.NewTokenMissionRepositoryImpl(db)
	tokenNotifSvc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)
	tokenMissionSvc := service.NewTokenMissionServiceImpl(tokenMissionRepo, nil, tokenAccountRepo)
	tokenAccountSvc := service.NewTokenAccountServiceImpl(tokenAccountRepo, tokenTopUpRepo, centerRepo, tokenMissionSvc, tokenNotifSvc)
	tokenMissionSvc = service.NewTokenMissionServiceImpl(tokenMissionRepo, tokenAccountSvc, tokenAccountRepo)

	svc := service.NewAuthAccountServiceImpl(authRepo, clientRepo, personnelRepo, centerRepo, sessionRepo, tokenRepo, mailer, tokenAccountSvc, validate)
	ctrl := controller.NewAuthAccountController(svc)

	// One limiter shared by every auth route below so a single attacker burns a
	// single budget across login/register/social/refresh/logout/logout-all and
	// the email flows. 429 + Retry-After on throttle; limits are env-configurable
	// (RATE_LIMIT_*) with production defaults.
	authLimiter := middleware.RateLimit(middleware.RateLimitConfigFromEnv())

	// Legacy routes under /auth-accounts. The email-verification and
	// password-recovery routes join this same group, so they inherit the shared
	// limiter (per-IP and per-email, since the limiter reads the "email" key).
	authAccount := rg.Group("/auth-accounts", authLimiter)
	authAccount.POST("/login", ctrl.Login)
	authAccount.POST("/social", ctrl.SocialLogin)
	authAccount.POST("/register", ctrl.Register)
	authAccount.POST("/refresh", ctrl.Refresh)
	authAccount.POST("/logout", middleware.JWTAuthMiddleware(), ctrl.Logout)
	authAccount.POST("/logout-all", middleware.JWTAuthMiddleware(), ctrl.LogoutAll)
	authAccount.POST("/verify-email", ctrl.VerifyEmail)
	authAccount.POST("/resend-verification", ctrl.ResendVerification)
	authAccount.POST("/forgot-password", ctrl.ForgotPassword)
	authAccount.POST("/reset-password", ctrl.ResetPassword)

	// Routes under /auth (used by mobile admin panel). The limiter runs before
	// the JWT check so throttling happens before any CPU is spent on the token.
	auth := rg.Group("/auth")
	auth.POST("/register-personnel", authLimiter, middleware.JWTAuthMiddleware(), ctrl.RegisterPersonnel)
}
