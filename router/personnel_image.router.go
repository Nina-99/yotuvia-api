package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/gin-gonic/gin"
)

func PersonnelImageRoutes(rg *gin.RouterGroup, personnelImageController *controller.PersonnelImageController) {
	personnelImages := rg.Group("/personnel-images", middleware.JWTAuthMiddleware())
	{
		personnelImages.POST("", personnelImageController.Create)
		personnelImages.GET("/:id", personnelImageController.FindByID)
		personnelImages.DELETE("/:id", personnelImageController.Delete)
		personnelImages.GET("/personnel/:id", personnelImageController.FindByPersonnelID)
	}
}
