package router

import (
	"github.com/Nina-99/yotuvia-api/controller"
	"github.com/Nina-99/yotuvia-api/middleware"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

func PersonnelServiceRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewPersonnelServiceRepositoryImpl(db)
	personnelRepo := repository.NewPersonnelRepositoryImpl(db)
	serviceRepo := repository.NewServiceRepositoryImpl(db)
	svc := service.NewPersonnelServiceServiceImpl(repo, personnelRepo, serviceRepo, validate)
	ctrl := controller.NewPersonnelServiceController(svc)

	ps := rg.Group("/personnel-services")
	// The capability table is a WRITE surface with security impact: assigning a
	// service grants a professional the ability to be booked for it. Every route
	// therefore requires an authenticated actor. Writes are additionally
	// center-only AND ownership-checked in the service (the acting centre must
	// own both the personnel and the service); the GET stays reachable by any
	// authenticated role because the booking picker runs on a client token.
	ps.Use(middleware.JWTAuthMiddleware())
	{
		ps.POST("", middleware.RequireRole("beauty_center"), ctrl.Create)
		ps.DELETE("/:id", middleware.RequireRole("beauty_center"), ctrl.Delete)
		ps.GET("", ctrl.List)
	}
}
