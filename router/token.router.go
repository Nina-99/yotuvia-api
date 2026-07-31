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

func TokenRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	// Repos
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	txRepo := repository.NewTokenTransactionRepositoryImpl(db)
	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	referralRepo := repository.NewReferralRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)

	// Services
	missionSvc := service.NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	txSvc := service.NewTokenTransactionServiceImpl(txRepo)
	referralSvc := service.NewReferralServiceImpl(referralRepo, centerRepo, nil)
	accountSvc := service.NewTokenAccountServiceImpl(accountRepo, topUpRepo, txRepo, centerRepo, missionSvc)

	// Wire circular refs
	missionSvc = service.NewTokenMissionServiceImpl(missionRepo, accountSvc, accountRepo)
	referralSvc = service.NewReferralServiceImpl(referralRepo, centerRepo, accountSvc)

	ctrl := controller.NewTokenController(accountSvc, txSvc, missionSvc, referralSvc, topUpRepo, centerRepo)

	tokens := rg.Group("/tokens")
	tokens.Use(middleware.JWTAuthMiddleware())
	{
		tokens.GET("/balance", ctrl.GetBalance)
		tokens.GET("/history", ctrl.GetHistory)
		tokens.POST("/topup", ctrl.CreateTopUp)
		tokens.GET("/missions", ctrl.GetMissions)
		tokens.POST("/missions/:id/claim", ctrl.ClaimMission)
		tokens.GET("/referral-info", ctrl.GetReferralInfo)
		tokens.POST("/referrals", ctrl.CreateReferral)
		tokens.GET("/referrals", ctrl.GetReferrals)
	}

	admin := rg.Group("/admin/tokens")
	admin.Use(middleware.JWTAuthMiddleware())
	{
		admin.GET("/topups", ctrl.AdminListTopUps)
		admin.PUT("/topups/:id", ctrl.AdminProcessTopUp)
	}
}
