package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/gin-gonic/gin"
)

func SocialMediaRoutes(rg *gin.RouterGroup, socialMediaController *controller.SocialMediaController) {
	// Public route
	rg.GET("/beauty-centers/:id/social-media", socialMediaController.FindByBeautyCenterID)

	// Protected routes
	socialMedia := rg.Group("/social-media")
	socialMedia.Use(middleware.JWTAuthMiddleware())
	{
		socialMedia.POST("", socialMediaController.Create)
		socialMedia.GET("/:id", socialMediaController.FindByID)
		socialMedia.PUT("/:id", socialMediaController.Update)
		socialMedia.DELETE("/:id", socialMediaController.Delete)
	}
}
