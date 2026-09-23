package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

func LoyaltyRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewLoyaltyRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	svc := service.NewLoyaltyService(repo, centerRepo,
		service.NewTokenMissionServiceImpl(
			repository.NewTokenMissionRepositoryImpl(db), nil,
			repository.NewTokenAccountRepositoryImpl(db),
		), validate)
	ctrl := controller.NewLoyaltyController(svc)

	loyalty := rg.Group("/loyalty")
	{
		loyalty.POST("", ctrl.CreateProgram)
		loyalty.GET("/client/:clientId", ctrl.GetCLientCards)
		loyalty.GET("/center/:centerId", ctrl.FindByCenter)
		loyalty.GET("/center/:centerId/settings", ctrl.GetCenterSettings)
		loyalty.PUT("/center/:centerId/settings", ctrl.UpdateCenterSettings)
		loyalty.PUT("/:id", ctrl.UpdateProgram)
		loyalty.DELETE("/:id", ctrl.DeleteProgram)
		loyalty.PATCH("/:id/toggle", ctrl.ToggleProgram)
	}
}
