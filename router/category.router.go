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

func CategoryRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewCategoryRepositoryImpl(db)
	svc := service.NewCategoryServiceImpl(repo, validate)
	ctrl := controller.NewCategoryController(svc)

	category := rg.Group("/category")
	category.Use(middleware.JWTAuthMiddleware())
	{
		// Writes are centre-admin only; reads stay open to any authenticated
		// caller. The service additionally proves the row belongs to the
		// caller's centre.
		category.POST("", middleware.RequireRole("beauty_center"), ctrl.Create)
		category.PUT("/:id", middleware.RequireRole("beauty_center"), ctrl.Update)
		category.DELETE("/:id", middleware.RequireRole("beauty_center"), ctrl.Delete)
		category.GET("/templates", ctrl.Templates)
		category.GET("", ctrl.FindAll)
		category.GET("/:id", ctrl.FindById)
	}
}
