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

// TimeBlockRoutes mounts the unavailability-block management for a professional
// (own blocks) or a centre owner (any professional of the centre, or a
// centre-wide block). Authentication + role gating live in the group; every
// semantic rule and ownership check lives in the service.
func TimeBlockRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewTimeBlockRepositoryImpl(db)
	personnelRepo := repository.NewPersonnelRepositoryImpl(db)
	svc := service.NewTimeBlockService(repo, personnelRepo, validate)
	ctrl := controller.NewTimeBlockController(svc)

	blocks := rg.Group("/time-blocks")
	blocks.Use(middleware.JWTAuthMiddleware())
	blocks.Use(middleware.RequireRole("personnel", "beauty_center"))
	{
		blocks.POST("", ctrl.Create)
		blocks.GET("", ctrl.List)
		blocks.DELETE("/:id", ctrl.Delete)
	}
}
