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
	repo := repository.NewAuthAccountRepositoryImpl(db)
	svc := service.NewAuthAccountServiceImpl(repo, validate)
	ctrl := controller.NewAuthAccountController(svc)

	authAccount := rg.Group("/auth-accounts")
	authAccount.Use(middleware.JWTAuthMiddleware())
	{
		authAccount.POST("", ctrl.Create)
		authAccount.GET("", ctrl.FindAll)
		authAccount.GET("/:id", ctrl.FindByID)
		authAccount.PUT("/:id", ctrl.Update)
		authAccount.DELETE("/:id", ctrl.Delete)
	}
}
