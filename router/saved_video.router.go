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

func SavedVideoRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewSavedVideoRepositoryImpl(db)
	svc := service.NewSavedVideoServiceImpl(repo, validate)
	ctrl := controller.NewSavedVideoController(svc)

	saved := rg.Group("/saved-videos")
	{
		// All routes require authentication
		saved.Use(middleware.JWTAuthMiddleware())
		{
			saved.POST("/toggle", ctrl.Toggle)
			saved.GET("/post/:post_id/check", ctrl.CheckIfSaved)
			saved.GET("", ctrl.ListByClient)
		}
	}
}
