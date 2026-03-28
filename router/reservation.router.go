package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/gin-gonic/gin"
)

func ReservationRoutes(rg *gin.RouterGroup, reservationController *controller.ReservationController) {
	reservation := rg.Group("/reservation")
	reservation.Use(middleware.JWTAuthMiddleware())
	{
		reservation.POST("/", reservationController.Create)
		reservation.PUT("/:id", reservationController.Update)
		reservation.DELETE("/:id", reservationController.Delete)
		reservation.GET("", reservationController.FindAll)
		reservation.GET("/:id", reservationController.FindById)
	}
}
