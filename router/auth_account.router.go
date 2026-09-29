package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

func AuthAccountRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	clientRepo := repository.NewClientRepositoryImpl(db)
	personnelRepo := repository.NewPersonnelRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	authRepo := repository.NewAuthAccountRepositoryImpl(db)
	sessionRepo := repository.NewUserSessionRepositoryImpl(db)

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

	svc := service.NewAuthAccountServiceImpl(authRepo, clientRepo, personnelRepo, centerRepo, sessionRepo, tokenAccountSvc, validate)
	ctrl := controller.NewAuthAccountController(svc)

	// One limiter shared by every auth route below so a single attacker burns a
	// single budget across login/register/social/refresh/logout.
	// 429 + Retry-After on throttle; limits are env-configurable
	// (RATE_LIMIT_*) with production defaults.
	authLimiter := middleware.RateLimit(middleware.RateLimitConfigFromEnv())

	// Legacy routes under /auth-accounts
	authAccount := rg.Group("/auth-accounts", authLimiter)
	authAccount.POST("/login", ctrl.Login)
	authAccount.POST("/social", ctrl.SocialLogin)
	authAccount.POST("/register", ctrl.Register)
	authAccount.POST("/refresh", ctrl.Refresh)
	authAccount.POST("/logout", middleware.JWTAuthMiddleware(), ctrl.Logout)

	// Routes under /auth (used by mobile admin panel). The limiter runs before
	// the JWT check so throttling happens before any CPU is spent on the token.
	auth := rg.Group("/auth")
	auth.POST("/register-personnel", authLimiter, middleware.JWTAuthMiddleware(), ctrl.RegisterPersonnel)
}
