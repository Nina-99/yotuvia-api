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

func VideoRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	videoRepo := repository.NewVideoRepositoryImpl(db)
	svc := service.NewVideoServiceImpl(videoRepo)
	ctrl := controller.NewVideoController(svc)

	// Public routes
	rg.GET("/videos/feed", ctrl.Feed)
	rg.GET("/videos/center/:centerId", ctrl.GetByCenter)
	rg.POST("/videos/:id/views", ctrl.IncrementViews)

	// Static file serving for streaming
	rg.Static("/videos/stream", "uploads/videos")

	// Protected routes — upload and delete
	protected := rg.Group("/videos")
	protected.Use(middleware.JWTAuthMiddleware())
	{
		protected.POST("/upload", ctrl.Upload)
		protected.DELETE("/:id", ctrl.Delete)
	}
}
