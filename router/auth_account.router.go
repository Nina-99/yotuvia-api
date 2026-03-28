package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/gin-gonic/gin"
)

func AuthAccountRoutes(rg *gin.RouterGroup, authAccountController *controller.AuthAccountController) {
	authAccount := rg.Group("/auth-accounts")
	authAccount.Use(middleware.JWTAuthMiddleware())
	{
		authAccount.POST("", authAccountController.Create)
		authAccount.GET("", authAccountController.FindAll)
		authAccount.GET("/:id", authAccountController.FindByID)
		authAccount.PUT("/:id", authAccountController.Update)
		authAccount.DELETE("/:id", authAccountController.Delete)
	}
}
