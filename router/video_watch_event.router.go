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

func VideoWatchEventRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	watchRepo := repository.NewVideoWatchEventRepositoryImpl(db)
	feedRepo := repository.NewFeedPostRepositoryImpl(db)
	likeRepo := repository.NewPostLikeRepositoryImpl(db)
	savedRepo := repository.NewSavedVideoRepositoryImpl(db)
	watchSvc := service.NewVideoWatchEventServiceImpl(watchRepo, feedRepo, likeRepo, savedRepo, validate)
	repoComment := repository.NewVideoCommentRepositoryImpl(db)
	feedSvc := service.NewFeedPostServiceImpl(feedRepo, nil, nil, nil, nil, nil, repoComment, validate)
	ctrl := controller.NewVideoWatchEventController(watchSvc, feedSvc)

	events := rg.Group("/video-watch-events")
	{
		events.Use(middleware.JWTAuthMiddleware())
		{
			events.POST("", ctrl.RecordWatch)
		}
	}

	// Personalized feed — protected
	personalized := rg.Group("/feed-posts")
	{
		personalized.Use(middleware.JWTAuthMiddleware())
		{
			personalized.GET("/personalized", ctrl.GetPersonalizedFeed)
		}
	}
}
