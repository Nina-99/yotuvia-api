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

// BoostRoutes mounts the center-facing CPM promotion endpoints.
func BoostRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	feedRepo := repository.NewFeedPostRepositoryImpl(db)
	boostRepo := repository.NewBoostCampaignRepositoryImpl(db)

	// The auction holds budgets through the token account service, wired the
	// same way as the token routes.
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

	boostSvc := service.NewBoostServiceImpl(boostRepo, feedRepo, accountSvc)
	ctrl := controller.NewBoostController(boostSvc)

	campaigns := rg.Group("/boost-campaigns")
	{
		campaigns.Use(middleware.JWTAuthMiddleware(), middleware.RequireRole("beauty_center"))
		{
			campaigns.POST("", ctrl.Create)
			campaigns.GET("", ctrl.List)
			campaigns.PUT("/:id/pause", ctrl.Pause)
		}
	}
}
