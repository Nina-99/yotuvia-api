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

func ReviewRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewReviewRepositoryImpl(db)
	notifSvc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)
	svc := service.NewReviewServiceImpl(repo, notifSvc,
		service.NewTokenMissionServiceImpl(
			repository.NewTokenMissionRepositoryImpl(db), nil,
			repository.NewTokenAccountRepositoryImpl(db),
		), validate)
	ctrl := controller.NewReviewController(svc)

	// Public routes. Reviews are read by anonymous visitors browsing a center:
	// the client object is TRIMMED to name + avatar (no email/phone/username),
	// so id enumeration cannot harvest PII.
	reviews := rg.Group("/reviews")
	{
		reviews.GET("/:id", ctrl.FindByID)
		reviews.GET("/beauty-center/:beautyCenterID", ctrl.FindByBeautyCenter)
	}

	// Protected routes (require JWT)
	protected := rg.Group("/reviews")
	protected.Use(middleware.JWTAuthMiddleware())
	{
		// Authenticated-only: a `client` token may read only its own id
		// (ownership enforced in the service, 403 on mismatch); beauty_center
		// and personnel keep the staff-facing read.
		protected.GET("/client/:clientID", ctrl.FindByClient)
		protected.POST("/", ctrl.Create)
		protected.PUT("/:id", ctrl.Update)
		protected.DELETE("/:id", ctrl.Delete)
		protected.GET("/can-review/:beautyCenterID", ctrl.CanReview)
	}
}
