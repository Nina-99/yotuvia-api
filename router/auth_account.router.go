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

	// Token service (for welcome bonus)
	tokenAccountRepo := repository.NewTokenAccountRepositoryImpl(db)
	tokenTopUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	tokenMissionRepo := repository.NewTokenMissionRepositoryImpl(db)
	tokenMissionSvc := service.NewTokenMissionServiceImpl(tokenMissionRepo, nil, tokenAccountRepo)
	tokenAccountSvc := service.NewTokenAccountServiceImpl(tokenAccountRepo, tokenTopUpRepo, centerRepo, tokenMissionSvc)
	tokenMissionSvc = service.NewTokenMissionServiceImpl(tokenMissionRepo, tokenAccountSvc, tokenAccountRepo)

	svc := service.NewAuthAccountServiceImpl(authRepo, clientRepo, personnelRepo, centerRepo, tokenAccountSvc, validate)
	ctrl := controller.NewAuthAccountController(svc)

	// Legacy routes under /auth-accounts
	authAccount := rg.Group("/auth-accounts")
	authAccount.POST("/login", ctrl.Login)
	authAccount.POST("/social", ctrl.SocialLogin)
	authAccount.POST("/register", ctrl.Register)
	authAccount.POST("/check-email", ctrl.CheckEmail)

	// Routes under /auth (used by mobile admin panel)
	auth := rg.Group("/auth")
	auth.POST("/register-personnel", middleware.JWTAuthMiddleware(), ctrl.RegisterPersonnel)
}
