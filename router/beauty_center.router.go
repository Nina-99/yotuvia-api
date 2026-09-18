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
	svc := service.NewBeautyCenterServiceImpl(repo,
		service.NewTokenMissionServiceImpl(
			repository.NewTokenMissionRepositoryImpl(db), nil,
			repository.NewTokenAccountRepositoryImpl(db),
		), validate)
	ctrl := controller.NewBeautyCenterController(svc)

	beautyCenter := rg.Group("/beauty-centers")
	{
		beautyCenter.GET("", ctrl.FindAll)
		beautyCenter.GET("/:id", ctrl.FindByID)
		beautyCenter.GET("/:id/available-slots", ctrl.GetAvailableSlots)
		beautyCenter.GET("/search", ctrl.Search)
		beautyCenter.GET("/top-rated", ctrl.FindTopRated)
		beautyCenter.GET("/nearby", ctrl.FindNearby)
		beautyCenter.GET("/ranking", ctrl.FindRanking)

		// Protected routes
		beautyCenter.Use(middleware.JWTAuthMiddleware())
		{
			beautyCenter.PUT("/:id", ctrl.Update)
			beautyCenter.DELETE("/:id", ctrl.Delete)
		}
	}
}
