package router

import (
	"github.com/Nina-99/yotuvia-api/controller"
	"github.com/Nina-99/yotuvia-api/middleware"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

// PlatformRoutes mounts the platform-level `super_admin` surface. Every route is
// gated by JWTAuthMiddleware + RequireRole("super_admin"), so the three existing
// roles (and anonymous callers) are rejected before any handler runs.
//
// The whole surface is composed from the SAME services the centre-scoped routes
// use: TokenAccountService for top-up decisions and PaymentConfigService +
// UploadController for payment config and QR upload. Nothing is re-implemented
// here — the platform view only differs in WHOSE data it is allowed to touch.
func PlatformRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	// Repos
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)

	// Services. The top-up decision path only needs the notification service
	// (approved/rejected push); it never walks the mission chain, so the circular
	// mission↔account wiring TokenRoutes performs is not required here.
	notifSvc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)
	missionSvc := service.NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	accountSvc := service.NewTokenAccountServiceImpl(accountRepo, topUpRepo, centerRepo, missionSvc, notifSvc)

	paymentSvc := service.NewPaymentConfigServiceImpl(repository.NewPaymentConfigRepositoryImpl(db))
	uploadCtrl := controller.NewUploadController()
	referralSvc := service.NewReferralServiceImpl(
		repository.NewReferralRepositoryImpl(db), centerRepo, nil, accountRepo,
		service.NewMarketConfigServiceImpl(repository.NewMarketConfigRepositoryImpl(db)),
	)
	beautyCenterSvc := service.NewBeautyCenterServiceImpl(
		centerRepo, missionSvc, validate, repository.NewTimeBlockRepositoryImpl(db),
		service.BeautyCenterDeps{ReferralService: referralSvc, TokenAccountRepo: accountRepo},
	)
	// Ads: the platform needs its own read-only view over the same auction data.
	boostSvc := service.NewBoostServiceImpl(
		repository.NewBoostCampaignRepositoryImpl(db),
		repository.NewFeedPostRepositoryImpl(db),
		accountSvc,
		accountRepo,
		service.NewMarketConfigServiceImpl(repository.NewMarketConfigRepositoryImpl(db)),
	)
	boostCtrl := controller.NewBoostController(boostSvc)

	// Feed quality: the platform reads the ranking's own instrument so a change
	// can be judged on data instead of on opinion.
	feedWatchSvc := service.NewVideoWatchEventServiceImpl(
		repository.NewVideoWatchEventRepositoryImpl(db),
		repository.NewFeedPostRepositoryImpl(db),
		repository.NewPostLikeRepositoryImpl(db),
		repository.NewSavedVideoRepositoryImpl(db),
		repository.NewFeedImpressionRepositoryImpl(db),
		validate,
	)
	feedCtrl := controller.NewVideoWatchEventController(feedWatchSvc, nil, nil)

	ctrl := controller.NewPlatformController(accountSvc, topUpRepo, centerRepo, paymentSvc, uploadCtrl, beautyCenterSvc)

	platform := rg.Group("/admin/platform")
	platform.Use(middleware.JWTAuthMiddleware(), middleware.RequireRole("super_admin"))
	{
		platform.GET("/tokens/topups", ctrl.ListTopUps)
		platform.PUT("/tokens/topups/:id", ctrl.ProcessTopUp)
		// Manual money path: pending backlog, aging and ledger drift.
		platform.GET("/tokens/reconciliation", ctrl.TopUpReconciliation)
		platform.GET("/centers", ctrl.ListCenters)
		// Advertising: the platform's revenue view.
		platform.GET("/ads/overview", boostCtrl.PlatformOverview)
		// Feed health: engagement per position, the baseline for ranking changes.
		platform.GET("/feed/metrics", feedCtrl.FeedMetrics)
		platform.PUT("/centers/:centerId/verify", ctrl.VerifyCenter)
		platform.GET("/centers/:centerId/payment", ctrl.GetCenterPayment)
		platform.PUT("/centers/:centerId/payment", ctrl.UpdateCenterPayment)
		platform.POST("/centers/:centerId/payment/qr", middleware.WithUploadGuard(ctrl.UploadCenterPaymentQR)...)
	}
}
