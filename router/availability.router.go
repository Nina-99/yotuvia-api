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

// AvailabilityRoutes mounts the public, client-facing cross-center
// availability search. It is a read path over the shared slot kernel, so it
// uses the same repositories as the per-center endpoint.
func AvailabilityRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	serviceRepo := repository.NewServiceRepositoryImpl(db)
	treatmentRepo := repository.NewTreatmentRepositoryImpl(db)
	personnelRepo := repository.NewPersonnelRepositoryImpl(db)
	svc := service.NewAvailabilityServiceImpl(centerRepo, serviceRepo, treatmentRepo, personnelRepo, repository.NewTimeBlockRepositoryImpl(db))
	ctrl := controller.NewAvailabilityController(svc)

	availability := rg.Group("/availability")
	// Public by design (a visitor sees availability before signing up) but
	// expensive per request: a dedicated budget keeps it from becoming a free
	// DoS amplifier or a competitor's agenda-scraping API.
	availability.Use(middleware.RateLimit(middleware.PublicSearchRateLimitConfig()))
	{
		availability.GET("/search", ctrl.Search)
	}
}
