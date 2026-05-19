package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

func ReservationRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewReservationRepositoryImpl(db)
	svc := service.NewReservationServiceImpl(repo, validate)
	ctrl := controller.NewReservationController(svc)

	reservation := rg.Group("/reservation")
	reservation.Use(middleware.JWTAuthMiddleware())
	{
		reservation.POST("/", ctrl.Create)
		reservation.PUT("/:id", ctrl.Update)
		reservation.DELETE("/:id", ctrl.Delete)
		reservation.GET("", ctrl.FindAll)
		reservation.GET("/:id", ctrl.FindById)
	}
}
