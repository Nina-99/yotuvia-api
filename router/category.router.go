package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/gin-gonic/gin"
)

func CategoryRoutes(rg *gin.RouterGroup, categoryController *controller.CategoryController) {
	category := rg.Group("/category")
	category.Use(middleware.JWTAuthMiddleware())
	{
		category.POST("", categoryController.Create)
		category.PUT("/:id", categoryController.Update)
		category.DELETE("/:id", categoryController.Delete)
		category.GET("", categoryController.FindAll)
		category.GET("/:id", categoryController.FindById)
	}
}
