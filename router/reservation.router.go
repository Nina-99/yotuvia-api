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

func ReservationRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	loyaltyRepo := repository.NewLoyaltyRepositoryImpl(db)
	tokenMissionRepo := repository.NewTokenMissionRepositoryImpl(db)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	loyaltySvc := service.NewLoyaltyService(loyaltyRepo,
		service.NewTokenMissionServiceImpl(tokenMissionRepo, nil, accountRepo), validate)

	serviceRepo := repository.NewServiceRepositoryImpl(db)
	repo := repository.NewReservationRepositoryImpl(db)
	personnelRepo := repository.NewPersonnelRepositoryImpl(db)
	notifRepo := repository.NewNotificationRepositoryImpl(db)
	pushRepo := repository.NewPushDeviceRepositoryImpl(db)
	notifSvc := service.NewNotificationServiceImpl(notifRepo, pushRepo)

	// Token services
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	missionSvc := service.NewTokenMissionServiceImpl(tokenMissionRepo, nil, accountRepo)
	accountSvc := service.NewTokenAccountServiceImpl(accountRepo, topUpRepo, centerRepo, missionSvc, notifSvc)
	missionSvc = service.NewTokenMissionServiceImpl(tokenMissionRepo, accountSvc, accountRepo)

	marketCfgSvc := service.NewMarketConfigServiceImpl(repository.NewMarketConfigRepositoryImpl(db))
	svc := service.NewReservationServiceImpl(repo, serviceRepo, personnelRepo, repository.NewClientRepositoryImpl(db), accountRepo, notifSvc, loyaltySvc, accountSvc, missionSvc, marketCfgSvc, validate)
	ctrl := controller.NewReservationController(svc)

	reservation := rg.Group("/reservation")
	reservation.Use(middleware.JWTAuthMiddleware())
	{
		// Client-only creation (actor fix): the client identity is resolved
		// from the JWT claim, never from the body. Admin walk-ins use
		// POST /reservation/admin below.
		reservation.POST("/", middleware.RequireRole("client"), ctrl.Create)
		// Admin walk-in booking: center admins and their staff book for a
		// provided active client. The service binds the body center against
		// the JWT centerID claim (design D2/D5).
		reservation.POST("/admin", middleware.RequireRole("beauty_center", "personnel"), ctrl.CreateAdmin)

		// client-only: owner of the reservation
		clientOnly := reservation.Group("/client", middleware.RequireRole("client"))
		{
			clientOnly.GET("/:clientId", ctrl.FindByClient)
			clientOnly.GET("/:clientId/upcoming", ctrl.FindUpcomingByClient)
		}
		reservation.PATCH("/:id/cancel", middleware.RequireRole("client"), ctrl.Cancel)

		// beauty_center-only (admin)
		admin := reservation.Group("", middleware.RequireRole("beauty_center"))
		{
			admin.GET("", ctrl.FindAll)
			admin.PATCH("/:id/status", ctrl.UpdateStatus)
			admin.PATCH("/:id/assign", ctrl.AssignPersonnel)
		}

		// personnel-only self-service agenda (design D1: claim-scoped, never a
		// path parameter). Static /me wins over /:id in Gin's tree.
		reservation.GET("/me", middleware.RequireRole("personnel"), ctrl.FindMyReservations)
		// personnel-only status transition on the staff's own reservations
		// (accept/reject pending turnos). Stays isolated from the admin
		// /:id/status group above (beauty_center-only).
		reservation.PATCH("/me/:id/status", middleware.RequireRole("personnel"), ctrl.UpdateMyStatus)

		// verify-qr widening lives in its OWN isolated group (design D4):
		// granting personnel access here must NOT leak /status or /assign,
		// which stay beauty_center-only above.
		verifyQR := reservation.Group("/verify-qr", middleware.RequireRole("beauty_center", "personnel"))
		{
			verifyQR.GET("", ctrl.VerifyQR)
		}

		// beauty_center + personnel (center staff)
		staff := reservation.Group("/center", middleware.RequireRole("beauty_center", "personnel"))
		{
			staff.GET("/:centerId", ctrl.FindByBeautyCenter)
		}

		// client + beauty_center (owner client OR owner center)
		owner := reservation.Group("", middleware.RequireRole("client", "beauty_center"))
		{
			owner.GET("/:id", ctrl.FindById)
			owner.PUT("/:id", ctrl.Update)
			owner.DELETE("/:id", ctrl.Delete)
			owner.POST("/generate-qr/:id", ctrl.GenerateQR)
		}
	}
}
