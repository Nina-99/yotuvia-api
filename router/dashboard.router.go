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

func DashboardRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewDashboardRepositoryImpl(db)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	notifSvc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)
	missionSvc := service.NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	accountSvc := service.NewTokenAccountServiceImpl(accountRepo, topUpRepo, centerRepo, missionSvc, notifSvc)
	svc := service.NewDashboardServiceImpl(repo, accountSvc)
	ctrl := controller.NewDashboardController(svc)

	dashboard := rg.Group("/dashboard", middleware.JWTAuthMiddleware(), middleware.RequireRole("beauty_center"))
	{
		dashboard.GET("/stats", ctrl.GetStats)
	}
}
