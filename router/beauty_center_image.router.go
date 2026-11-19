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

func BeautyCenterImageRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewBeautyCenterImageRepositoryImpl(db)
	repoBC := repository.NewBeautyCenterRepositoryImpl(db)
	svc := service.NewBeautyCenterImageServiceImpl(repo, repoBC, validate)
	ctrl := controller.NewBeautyCenterImageController(svc)

	beautyCenterImage := rg.Group("/beauty-image")
	beautyCenterImage.Use(middleware.JWTAuthMiddleware())
	// RequireRole proves the ROLE (beauty_center only — previously any
	// authenticated token, including `client`, could manage any centre's
	// gallery). Row ownership is enforced in the service against the centerID
	// claim, and Create scopes the body id to the claim in the controller.
	beautyCenterImage.Use(middleware.RequireRole("beauty_center"))
	{
		beautyCenterImage.POST("/upload", middleware.WithUploadGuard(ctrl.Upload)...)
		beautyCenterImage.POST("", ctrl.Create)
		beautyCenterImage.PATCH("/reorder", ctrl.Reorder)
		beautyCenterImage.PATCH("/:id", ctrl.Update)
		beautyCenterImage.GET("/:id", ctrl.FindByID)
		beautyCenterImage.DELETE("/:id", ctrl.Delete)
		beautyCenterImage.GET("/beauty_center/:id", ctrl.FindByBeautyCenterID)
	}
}
