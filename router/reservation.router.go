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
	loyaltySvc := service.NewLoyaltyService(loyaltyRepo, validate)

	serviceRepo := repository.NewServiceRepositoryImpl(db)
	repo := repository.NewReservationRepositoryImpl(db)
	personnelRepo := repository.NewPersonnelRepositoryImpl(db)
	notifRepo := repository.NewNotificationRepositoryImpl(db)
	pushRepo := repository.NewPushDeviceRepositoryImpl(db)
	notifSvc := service.NewNotificationServiceImpl(notifRepo, pushRepo)

	// Token services
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	txRepo := repository.NewTokenTransactionRepositoryImpl(db)
	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	missionSvc := service.NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	accountSvc := service.NewTokenAccountServiceImpl(accountRepo, topUpRepo, txRepo, centerRepo, missionSvc)
	missionSvc = service.NewTokenMissionServiceImpl(missionRepo, accountSvc, accountRepo)

	svc := service.NewReservationServiceImpl(repo, serviceRepo, personnelRepo, notifSvc, loyaltySvc, accountSvc, missionSvc, validate)
	ctrl := controller.NewReservationController(svc)

	reservation := rg.Group("/reservation")
	reservation.Use(middleware.JWTAuthMiddleware())
	{
		reservation.POST("/", ctrl.Create)

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
			admin.GET("/verify-qr", ctrl.VerifyQR)
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
