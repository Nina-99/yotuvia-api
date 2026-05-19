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

func SocialMediaRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewSocialMediaRepositoryImpl(db)
	svc := service.NewSocialMediaServiceImpl(repo, validate)
	ctrl := controller.NewSocialMediaController(svc)

	// Public route
	rg.GET("/beauty-centers/:id/social-media", ctrl.FindByBeautyCenterID)

	// Protected routes
	socialMedia := rg.Group("/social-media")
	socialMedia.Use(middleware.JWTAuthMiddleware())
	{
		socialMedia.POST("", ctrl.Create)
		socialMedia.GET("/:id", ctrl.FindByID)
		socialMedia.PUT("/:id", ctrl.Update)
		socialMedia.DELETE("/:id", ctrl.Delete)
	}
}
