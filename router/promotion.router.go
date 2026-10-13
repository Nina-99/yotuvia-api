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

func PromotionRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewPromotionRepositoryImpl(db)
	svc := service.NewPromotionService(repo, validate)
	ctrl := controller.NewPromotionController(svc)

	promos := rg.Group("/promotions")
	{
		// Public: list applicable promos for a service (client-facing)
		promos.GET("/center/:centerId/applicable", ctrl.FindApplicable)

		// Auth: center admin management. RequireRole proves the ROLE
		// (beauty_center only — previously any authenticated token, including
		// `client`, could manage promotions); the centerID claim is verified
		// against row ownership in the service layer, so a center-A admin
		// cannot touch center-B data.
		admin := promos.Group("")
		admin.Use(middleware.JWTAuthMiddleware())
		admin.Use(middleware.RequireRole("beauty_center"))
		{
			admin.POST("", ctrl.Create)
			admin.GET("/center/:centerId", ctrl.FindByCenter)
			admin.GET("/:id", ctrl.FindByID)
			admin.PUT("/:id", ctrl.Update)
			admin.DELETE("/:id", ctrl.Delete)
		}
	}
}
