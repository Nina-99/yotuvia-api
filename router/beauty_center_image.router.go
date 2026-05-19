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

func BeautyCenterImageRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewBeautyCenterImageRepositoryImpl(db)
	repoBC := repository.NewBeautyCenterRepositoryImpl(db)
	svc := service.NewBeautyCenterImageServiceImpl(repo, repoBC, validate)
	ctrl := controller.NewBeautyCenterImageController(svc)

	beautyCenterImage := rg.Group("/beauty-image", middleware.JWTAuthMiddleware())
	{
		beautyCenterImage.POST("", ctrl.Create)
		beautyCenterImage.GET("/:id", ctrl.FindByID)
		beautyCenterImage.DELETE("/:id", ctrl.Delete)
		beautyCenterImage.GET("/beauty_center/:id", ctrl.FindByBeautyCenterID)
	}
}
