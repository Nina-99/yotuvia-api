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

func PromotionRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewPromotionRepositoryImpl(db)
	svc := service.NewPromotionService(repo, validate)
	ctrl := controller.NewPromotionController(svc)

	promos := rg.Group("/promotions")
	{
		// Public: list applicable promos for a service (client-facing)
		promos.GET("/center/:centerId/applicable", ctrl.FindApplicable)

		// Auth: center admin management
		admin := promos.Group("")
		admin.Use(middleware.JWTAuthMiddleware())
		{
			admin.POST("", ctrl.Create)
			admin.GET("/center/:centerId", ctrl.FindByCenter)
			admin.GET("/:id", ctrl.FindByID)
			admin.PUT("/:id", ctrl.Update)
			admin.DELETE("/:id", ctrl.Delete)
		}
	}
}
