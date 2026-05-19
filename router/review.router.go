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

func ReviewRoutes(r *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewReviewRepositoryImpl(db)
	svc := service.NewReviewServiceImpl(repo, validate)
	ctrl := controller.NewReviewController(svc)

	reviews := r.Group("/reviews")
	{
		reviews.GET("/:id", ctrl.FindByID)
		reviews.GET("/beauty-center/:beautyCenterID", ctrl.FindByBeautyCenter)
		reviews.GET("/client/:clientID", ctrl.FindByClient)
	}

	protected := r.Group("/reviews")
	protected.Use(middleware.JWTAuthMiddleware())
	{
		protected.POST("/", ctrl.Create)
		protected.PUT("/:id", ctrl.Update)
		protected.DELETE("/:id", ctrl.Delete)
	}
}
