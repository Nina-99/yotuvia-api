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

func ServiceRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewServiceRepositoryImpl(db)
	repoC := repository.NewCategoryRepositoryImpl(db)
	svc := service.NewServiceServiceImpl(repo, repoC, validate)
	ctrl := controller.NewServiceController(svc)

	services := rg.Group("/services")
	services.Use(middleware.JWTAuthMiddleware())
	{
		services.POST("", ctrl.Create)
		services.PUT("/:id", ctrl.Update)
		services.DELETE("/:id", ctrl.Delete)
		services.GET("", ctrl.FindAll)
		services.GET("/templates", ctrl.Templates)
		services.GET("/:id", ctrl.FindById)
	}
}
