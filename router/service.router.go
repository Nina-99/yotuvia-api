package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/gin-gonic/gin"
)

func ServiceRoutes(rg *gin.RouterGroup, serviceController *controller.ServiceController) {
	services := rg.Group("/services")
	services.Use(middleware.JWTAuthMiddleware())
	{
		services.POST("", serviceController.Create)
		services.PUT("/:id", serviceController.Update)
		services.DELETE("/:id", serviceController.Delete)
		services.GET("", serviceController.FindAll)
		services.GET("/:id", serviceController.FindById)
	}
}
