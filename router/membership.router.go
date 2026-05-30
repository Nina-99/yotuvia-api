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

func MembershipRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	membershipRepo := repository.NewMembershipRepositoryImpl(db)
	centerMembershipRepo := repository.NewCenterMembershipRepositoryImpl(db)
	centerMembershipSvc := service.NewCenterMembershipServiceImpl(membershipRepo, centerMembershipRepo, validate)
	membershipSvc := service.NewMembershipService(membershipRepo, validate)
	ctrl := controller.NewMembershipController(membershipSvc, centerMembershipSvc)

	membership := rg.Group("/memberships")
	{
		membership.Use(middleware.JWTAuthMiddleware())
		{
			membership.POST("", ctrl.Create)
			membership.PUT("/:id", ctrl.Update)
			membership.DELETE("/:id", ctrl.Delete)
			membership.GET("/:id", ctrl.FindByID)
			membership.GET("", ctrl.FindAll)
			membership.POST("/buy", ctrl.Buy)
			membership.POST("/review/:id", ctrl.Review)
		}
	}
}
