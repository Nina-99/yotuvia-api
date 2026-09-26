package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

// AvailabilityRoutes mounts the public, client-facing cross-center
// availability search. It is a read path over the shared slot kernel, so it
// uses the same repositories as the per-center endpoint.
func AvailabilityRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	serviceRepo := repository.NewServiceRepositoryImpl(db)
	svc := service.NewAvailabilityServiceImpl(centerRepo, serviceRepo)
	ctrl := controller.NewAvailabilityController(svc)

	availability := rg.Group("/availability")
	{
		availability.GET("/search", ctrl.Search)
	}
}
