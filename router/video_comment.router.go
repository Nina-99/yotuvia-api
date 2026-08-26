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

func VideoCommentRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	commentRepo := repository.NewVideoCommentRepositoryImpl(db)
	feedPostRepo := repository.NewFeedPostRepositoryImpl(db)
	clientRepo := repository.NewClientRepositoryImpl(db)

	svc := service.NewVideoCommentServiceImpl(commentRepo, feedPostRepo, clientRepo)
	ctrl := controller.NewVideoCommentController(svc)

	comments := rg.Group("/feed-posts/:id/comments")
	{
		comments.Use(middleware.JWTAuthMiddleware())
		comments.POST("", ctrl.CreateComment)
		comments.GET("", ctrl.ListComments)
		comments.GET("/:cid", ctrl.GetCommentReplies)
		comments.POST("/:cid/reply", ctrl.ReplyToComment)
		comments.PATCH("/:cid", ctrl.UpdateComment)
		comments.DELETE("/:cid", ctrl.DeleteComment)
		comments.PATCH("/:cid/hide", ctrl.HideComment)
	}
}
