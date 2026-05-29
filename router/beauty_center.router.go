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

func BeautyCenterRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewBeautyCenterRepositoryImpl(db)
	svc := service.NewBeautyCenterServiceImpl(repo, validate)
	ctrl := controller.NewBeautyCenterController(svc)

	beautyCenter := rg.Group("/beauty-centers")
	{
		beautyCenter.GET("", ctrl.FindAll)
		beautyCenter.GET("/:id", ctrl.FindByID)

		// Protected routes
		beautyCenter.Use(middleware.JWTAuthMiddleware())
		{
			beautyCenter.PUT("/:id", ctrl.Update)
			beautyCenter.DELETE("/:id", ctrl.Delete)
		}
	}
}
