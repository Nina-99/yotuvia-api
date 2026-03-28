package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/gin-gonic/gin"
)

func ReviewRouter(r *gin.RouterGroup, ctrl *controller.ReviewController) {
	reviews := r.Group("/reviews")
	{
		reviews.GET("/:id", ctrl.FindByID)
		reviews.GET("/beauty-center/:beautyCenterID", ctrl.FindByBeautyCenter)
		reviews.GET("/client/:clientID", ctrl.FindByClient)
	}

	protected := r.Group("/reviews")
	protected.Use(middleware.JWTAuthMiddleware())
	{
		protected.POST("/", ctrl.Create)
		protected.PUT("/:id", ctrl.Update)
		protected.DELETE("/:id", ctrl.Delete)
	}
}
