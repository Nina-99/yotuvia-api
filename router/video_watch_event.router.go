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

func VideoWatchEventRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	watchRepo := repository.NewVideoWatchEventRepositoryImpl(db)
	feedRepo := repository.NewFeedPostRepositoryImpl(db)
	likeRepo := repository.NewPostLikeRepositoryImpl(db)
	savedRepo := repository.NewSavedVideoRepositoryImpl(db)
	watchSvc := service.NewVideoWatchEventServiceImpl(watchRepo, feedRepo, likeRepo, savedRepo, validate)

	// FeedPostServiceImpl must be wired with the same real repositories as
	// FeedPostRoutes. FindAll dereferences PostLikeRepository and
	// SavedVideoRepository whenever a client id is present, and the
	// personalized endpoint is JWT-protected, so those paths are always
	// reached. Passing nil here caused a runtime nil-pointer 500.
	bcRepo := repository.NewBeautyCenterRepositoryImpl(db)
	personnelRepo := repository.NewPersonnelRepositoryImpl(db)
	serviceRepo := repository.NewServiceRepositoryImpl(db)
	repoComment := repository.NewVideoCommentRepositoryImpl(db)
	followRepo := repository.NewPersonnelFollowRepositoryImpl(db)
	notifSvc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)
	feedSvc := service.NewFeedPostServiceImpl(feedRepo, bcRepo, personnelRepo, serviceRepo, likeRepo, savedRepo, repoComment, followRepo, notifSvc, validate)

	// Boost (CPM promotion). The auction holds budgets through the token account
	// service, so it is wired exactly like the token routes.
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	missionSvc := service.NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	accountSvc := service.NewTokenAccountServiceImpl(accountRepo, topUpRepo, bcRepo, missionSvc, notifSvc)
	missionSvc = service.NewTokenMissionServiceImpl(missionRepo, accountSvc, accountRepo)
	boostSvc := service.NewBoostServiceImpl(repository.NewBoostCampaignRepositoryImpl(db), feedRepo, accountSvc)

	ctrl := controller.NewVideoWatchEventController(watchSvc, feedSvc, boostSvc)

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
