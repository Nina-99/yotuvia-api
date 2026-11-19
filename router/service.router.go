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

func ServiceRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewServiceRepositoryImpl(db)
	repoC := repository.NewCategoryRepositoryImpl(db)
	svc := service.NewServiceServiceImpl(repo, repoC, validate)
	ctrl := controller.NewServiceController(svc)

	services := rg.Group("/services")
	services.Use(middleware.JWTAuthMiddleware())
	{
		// Writes are centre-admin only. Reads stay open to any authenticated
		// caller (clients browse catalogs); RequireRole is applied per route so
		// gating the writes does not lock clients out of the listing.
		services.POST("", middleware.RequireRole("beauty_center"), ctrl.Create)
		services.PUT("/:id", middleware.RequireRole("beauty_center"), ctrl.Update)
		services.DELETE("/:id", middleware.RequireRole("beauty_center"), ctrl.Delete)
		services.GET("", ctrl.FindAll)
		services.GET("/templates", ctrl.Templates)
		services.GET("/:id", ctrl.FindById)
	}
}
