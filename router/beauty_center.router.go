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

func BeautyCenterRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewBeautyCenterRepositoryImpl(db)
	svc := service.NewBeautyCenterServiceImpl(repo,
		service.NewTokenMissionServiceImpl(
			repository.NewTokenMissionRepositoryImpl(db), nil,
			repository.NewTokenAccountRepositoryImpl(db),
		), validate, repository.NewTimeBlockRepositoryImpl(db))
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

		// Protected routes: identity + role + ownership. RequireRole proves the
		// ROLE (beauty_center only — previously any authenticated token,
		// including `client`, could rewrite or delete any centre); the service
		// then enforces id == the caller's centerID claim so a center-A admin
		// cannot touch center B.
		beautyCenter.Use(middleware.JWTAuthMiddleware())
		beautyCenter.Use(middleware.RequireRole("beauty_center"))
		{
			beautyCenter.PUT("/:id", ctrl.Update)
			beautyCenter.DELETE("/:id", ctrl.Delete)
		}
	}
}
