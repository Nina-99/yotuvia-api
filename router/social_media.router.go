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

func SocialMediaRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewSocialMediaRepositoryImpl(db)
	svc := service.NewSocialMediaServiceImpl(repo, validate)
	ctrl := controller.NewSocialMediaController(svc)

	// Public route
	rg.GET("/beauty-centers/:id/social-media", ctrl.FindByBeautyCenterID)

	// Protected routes: identity + role + ownership. RequireRole proves the
	// ROLE (beauty_center only — previously any authenticated token, including
	// `client`, could create/update/delete any centre's links). The service
	// enforces row ownership against the centerID claim and Create scopes the
	// body id to the claim in the controller.
	socialMedia := rg.Group("/social-media")
	socialMedia.Use(middleware.JWTAuthMiddleware())
	socialMedia.Use(middleware.RequireRole("beauty_center"))
	{
		socialMedia.POST("", ctrl.Create)
		socialMedia.GET("/:id", ctrl.FindByID)
		socialMedia.PUT("/:id", ctrl.Update)
		socialMedia.DELETE("/:id", ctrl.Delete)
	}
}
