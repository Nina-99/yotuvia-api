package router

import (
	"github.com/Nina-99/yottu-api/controller"
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
	svc := service.NewAuthAccountServiceImpl(authRepo, clientRepo, personnelRepo, centerRepo, validate)
	ctrl := controller.NewAuthAccountController(svc)

	authAccount := rg.Group("/auth-accounts")
	authAccount.POST("/login", ctrl.Login)
	authAccount.POST("/Social", ctrl.SocialLogin)
	authAccount.POST("/register", ctrl.Register)
}
