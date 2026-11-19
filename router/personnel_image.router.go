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

func PersonnelImageRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewPersonnelImageRepositoryImpl(db)
	repoP := repository.NewPersonnelRepositoryImpl(db)
	svc := service.NewPersonnelImageServiceImpl(repo, repoP, validate)
	ctrl := controller.NewPersonnelImageController(svc)
	personnelImages := rg.Group("/personnel-images", middleware.JWTAuthMiddleware())
	{
		personnelImages.POST("/upload", middleware.WithUploadGuard(ctrl.Upload)...)
		personnelImages.POST("", ctrl.Create)
		personnelImages.GET("/:id", ctrl.FindByID)
		personnelImages.DELETE("/:id", ctrl.Delete)
		personnelImages.GET("/personnel/:id", ctrl.FindByPersonnelID)
	}
}
