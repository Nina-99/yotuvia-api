package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/gin-gonic/gin"
)

func MembershipRoutes(rg *gin.RouterGroup, membershipController *controller.MembershipController) {
	membership := rg.Group("/memberships")
	{
		membership.Use(middleware.JWTAuthMiddleware())
		{
			membership.POST("", membershipController.Create)
			membership.PUT("/:id", membershipController.Update)
			membership.DELETE("/:id", membershipController.Delete)
			membership.GET("/:id", membershipController.FindByID)
			membership.GET("", membershipController.FindAll)
		}
	}
}
