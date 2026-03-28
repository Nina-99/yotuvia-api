package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/gin-gonic/gin"
)

func FeedPostRoutes(rg *gin.RouterGroup, feedPostController *controller.FeedPostController) {
	posts := rg.Group("/feed-posts")
	{
		posts.GET("", feedPostController.FindAll)
		posts.GET("/:id", feedPostController.FindByID)

		posts.Use(middleware.JWTAuthMiddleware())
		{
			posts.POST("", feedPostController.Create)
			posts.PUT("/:id", feedPostController.Update)
			posts.DELETE("/:id", feedPostController.Delete)
			posts.GET("/center/:id", feedPostController.FindByBeautyCenterID)
		}
	}
}
