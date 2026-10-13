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

func PersonnelInvitationRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	invitationRepo := repository.NewPersonnelInvitationRepositoryImpl(db)
	personnelRepo := repository.NewPersonnelRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	authRepo := repository.NewAuthAccountRepositoryImpl(db)
	svc := service.NewPersonnelInvitationServiceImpl(invitationRepo, personnelRepo, centerRepo, authRepo, validate)
	ctrl := controller.NewPersonnelInvitationController(svc)

	// Public routes (no auth required)
	rg.GET("/personnel/invite/:token", ctrl.GetByToken)
	rg.POST("/personnel/accept-invite", ctrl.Accept)

	// Protected routes (JWT required — center only)
	protected := rg.Group("/personnel/invitations")
	protected.Use(middleware.JWTAuthMiddleware())
	{
		protected.POST("", ctrl.Generate)
		protected.GET("", ctrl.ListByCenter)
	}
}
