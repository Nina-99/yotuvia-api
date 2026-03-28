package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/gin-gonic/gin"
)

func BeautyCenterRoutes(rg *gin.RouterGroup, beautyCenterController *controller.BeautyCenterController) {
	beautyCenter := rg.Group("/beauty-centers")
	{
		beautyCenter.GET("", beautyCenterController.FindAll)
		beautyCenter.GET("/:id", beautyCenterController.FindByID)

		// Protected routes
		beautyCenter.Use(middleware.JWTAuthMiddleware())
		{
			beautyCenter.POST("", beautyCenterController.Create)
			beautyCenter.PUT("/:id", beautyCenterController.Update)
			beautyCenter.DELETE("/:id", beautyCenterController.Delete)
		}
	}
}
