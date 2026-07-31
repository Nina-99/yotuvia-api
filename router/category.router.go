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

func CategoryRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewCategoryRepositoryImpl(db)
	svc := service.NewCategoryServiceImpl(repo, validate)
	ctrl := controller.NewCategoryController(svc)

	category := rg.Group("/category")
	category.Use(middleware.JWTAuthMiddleware())
	{
		category.POST("", ctrl.Create)
		category.PUT("/:id", ctrl.Update)
		category.DELETE("/:id", ctrl.Delete)
		category.GET("/templates", ctrl.Templates)
		category.GET("", ctrl.FindAll)
		category.GET("/:id", ctrl.FindById)
	}
}
