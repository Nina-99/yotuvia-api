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
		reservation.PUT("/:id", ctrl.Update)
		reservation.DELETE("/:id", ctrl.Delete)
		reservation.GET("", ctrl.FindAll)
		reservation.GET("/client/:clientId", ctrl.FindByClient)
		reservation.GET("/client/:clientId/upcoming", ctrl.FindUpcomingByClient)
		reservation.GET("/center/:centerId", ctrl.FindByBeautyCenter)
		reservation.GET("/:id", ctrl.FindById)
		reservation.PATCH("/:id/status", ctrl.UpdateStatus)
		reservation.PATCH("/:id/assign", ctrl.AssignPersonnel)
		reservation.PATCH("/:id/cancel", ctrl.Cancel)
		reservation.POST("/generate-qr/:id", ctrl.GenerateQR)
		reservation.GET("/verify-qr", ctrl.VerifyQR)
	}
}
