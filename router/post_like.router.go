package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/gin-gonic/gin"
)

func PostLikeRoutes(rg *gin.RouterGroup, postLikeController *controller.PostLikeController) {
	likes := rg.Group("/post-likes")
	{
		// Public route? (Check if user liked could be public but it needs clientID from JWT)
		// List likes is public
		likes.GET("/post/:post_id", postLikeController.GetLikesByPost)

		// Protected routes
		likes.Use(middleware.JWTAuthMiddleware())
		{
			likes.POST("", postLikeController.Create)
			likes.DELETE("/post/:post_id", postLikeController.Delete)
			likes.GET("/post/:post_id/check", postLikeController.CheckIfUserLiked)
		}
	}
}
