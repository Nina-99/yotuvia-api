package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/gin-gonic/gin"
)

func FavoriteRouter(router *gin.RouterGroup, favoriteController *controller.FavoriteController) {
	favorites := router.Group("/favorites").Use(middleware.JWTAuthMiddleware())
	{
		favorites.POST("", favoriteController.Create)
		favorites.DELETE("/:id", favoriteController.Delete)
	}
	clients := router.Group("/clients").Use(middleware.JWTAuthMiddleware())
	{
		clients.GET("/:client_id/favorites", favoriteController.FindByClientID)
	}
}
