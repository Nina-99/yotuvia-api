package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/gin-gonic/gin"
)

func PersonnelRoutes(rg *gin.RouterGroup, personnelController *controller.PersonnelController) {
	personnel := rg.Group("/personnel")
	{
		personnel.POST("", personnelController.Create)
		personnel.GET("/:id", personnelController.FindByID)
		personnel.GET("", personnelController.FindAll)
		personnel.PUT("/:id", personnelController.Update)
		personnel.DELETE("/:id", personnelController.Delete)
	}
}
