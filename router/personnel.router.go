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

func PersonnelRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewPersonnelRepositoryImpl(db)
	svc := service.NewPersonnelServiceImpl(repo, validate)
	ctrl := controller.NewPersonnelController(svc)

	personnel := rg.Group("/personnel")
	personnel.Use(middleware.JWTAuthMiddleware())
	{
		personnel.GET("/:id", ctrl.FindByID)
		personnel.GET("", ctrl.FindAll)
		personnel.PUT("/:id", middleware.RequireRole("beauty_center"), ctrl.Update)
		personnel.DELETE("/:id", middleware.RequireRole("beauty_center"), ctrl.Delete)
		personnel.PATCH("/me", middleware.RequireRole("personnel"), ctrl.UpdateMe)
	}
}
