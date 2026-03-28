package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/gin-gonic/gin"
)

func ClientRoutes(rg *gin.RouterGroup, clientController *controller.ClientController) {
	client := rg.Group("/client")
	client.POST("/register", clientController.Register)
	client.POST("/login", clientController.Login)
	client.Use(middleware.JWTAuthMiddleware())
	{
		client.GET("", clientController.FindAll)
		client.DELETE("/:id", clientController.Delete)
		client.PUT("/:id", clientController.Update)
	}
}
