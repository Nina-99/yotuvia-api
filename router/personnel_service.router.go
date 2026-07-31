package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
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
	{
		ps.POST("", ctrl.Create)
		ps.DELETE("/:id", ctrl.Delete)
		ps.GET("", ctrl.List)
	}
}
