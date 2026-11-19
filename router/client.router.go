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

func ClientRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewClientRepositoryImpl(db)
	svc := service.NewClientServiceImpl(repo, repository.NewUserSessionRepositoryImpl(db), validate)
	ctrl := controller.NewClientController(svc)

	client := rg.Group("/client")
	client.Use(middleware.JWTAuthMiddleware())
	{
		client.GET("/me", ctrl.FindMe)
		client.GET("", middleware.RequireRole("beauty_center"), ctrl.FindAllAdmin)
		client.GET("/:id", ctrl.FindByID)
		client.DELETE("/:id", ctrl.Delete)
		client.PUT("/:id", ctrl.Update)
		client.POST("/avatar", middleware.WithUploadGuard(middleware.RequireRole("client"), ctrl.UploadAvatar)...)
	}
}
