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

func PersonnelRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewPersonnelRepositoryImpl(db)
	svc := service.NewPersonnelServiceImpl(repo, repository.NewServiceRepositoryImpl(db), validate)
	followRepo := repository.NewPersonnelFollowRepositoryImpl(db)
	followSvc := service.NewPersonnelFollowServiceImpl(followRepo, repo, validate)
	ctrl := controller.NewPersonnelController(svc, followSvc)

	personnel := rg.Group("/personnel")
	personnel.Use(middleware.JWTAuthMiddleware())
	{
		personnel.GET("/:id", ctrl.FindByID)
		personnel.GET("", ctrl.FindAll)
		personnel.PUT("/:id", middleware.RequireRole("beauty_center"), ctrl.Update)
		personnel.DELETE("/:id", middleware.RequireRole("beauty_center"), ctrl.Delete)
		personnel.PATCH("/me", middleware.RequireRole("personnel"), ctrl.UpdateMe)
		// Follow a professional: client-only, actor from the JWT claim.
		personnel.POST("/:id/follow", middleware.RequireRole("client"), ctrl.Follow)
		personnel.DELETE("/:id/follow", middleware.RequireRole("client"), ctrl.Unfollow)
	}
}
