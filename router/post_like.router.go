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

func PostLikeRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewPostLikeRepositoryImpl(db)
	svc := service.NewPostLikeServiceImpl(repo, validate)
	ctrl := controller.NewPostLikeController(svc)

	likes := rg.Group("/post-likes")
	{
		// Public route? (Check if user liked could be public but it needs clientID from JWT)
		// List likes is public
		likes.GET("/post/:post_id", ctrl.GetLikesByPost)

	// Protected routes
	likes.Use(middleware.JWTAuthMiddleware())
	{
		likes.POST("", ctrl.Create)
		likes.POST("/toggle", ctrl.Toggle)
		likes.DELETE("/post/:post_id", ctrl.Delete)
		likes.GET("/post/:post_id/check", ctrl.CheckIfUserLiked)
	}
	}
}
