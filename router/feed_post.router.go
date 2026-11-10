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

// NewFeedPostService wires the feed service exactly once, so every router that
// needs it (the feed routes and the TikTok import path) shares the same wiring
// instead of duplicating the dependency list.
func NewFeedPostService(db *gorm.DB, validate *validator.Validate) service.FeedPostService {
	missionSvc := service.NewTokenMissionServiceImpl(
		repository.NewTokenMissionRepositoryImpl(db),
		nil,
		repository.NewTokenAccountRepositoryImpl(db),
	)
	return service.NewFeedPostServiceImpl(
		repository.NewFeedPostRepositoryImpl(db),
		repository.NewBeautyCenterRepositoryImpl(db),
		repository.NewPersonnelRepositoryImpl(db),
		repository.NewServiceRepositoryImpl(db),
		repository.NewPostLikeRepositoryImpl(db),
		repository.NewSavedVideoRepositoryImpl(db),
		repository.NewVideoCommentRepositoryImpl(db),
		repository.NewPersonnelFollowRepositoryImpl(db),
		service.NewNotificationServiceImpl(
			repository.NewNotificationRepositoryImpl(db),
			repository.NewPushDeviceRepositoryImpl(db),
		),
		validate,
		nil,
		missionSvc,
	)
}

func FeedPostRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	svc := NewFeedPostService(db, validate)
	ctrl := controller.NewFeedPostController(svc)

	posts := rg.Group("/feed-posts")
	{
		// Public: unified feed (active posts only) — optional auth for personalization.
		posts.GET("", middleware.OptionalJWTAuthMiddleware(), ctrl.FindAll)
		posts.POST("/:id/views", ctrl.IncrementViews)
		// Public: book-CTA tap counter, mirrors /views.
		posts.POST("/:id/book-taps", ctrl.IncrementBookTaps)
		posts.GET("/center/:id", ctrl.FindByBeautyCenterID)
		// Public: one professional's active videos, newest first.
		posts.GET("/personnel/:id", middleware.OptionalJWTAuthMiddleware(), ctrl.FindByPersonnelID)

		protected := posts.Group("")
		protected.Use(middleware.JWTAuthMiddleware())
		{
			// staff + beauty_center creators (role-scoped in service).
			protected.POST("", middleware.RequireRole("personnel", "beauty_center"), ctrl.Create)
			// Link an already-published external video instead of uploading.
			protected.POST("/external", middleware.RequireRole("personnel", "beauty_center"), ctrl.CreateExternal)
			protected.PUT("/:id", ctrl.Update)
			protected.DELETE("/:id", ctrl.Delete)
			protected.GET("/mine", ctrl.FindMine)

			// Center moderation over staff videos of their own center.
			suspend := protected.Group("")
			suspend.Use(middleware.RequireRole("beauty_center"))
			{
				suspend.PUT(":id/suspend", ctrl.Suspend)
			}
		}
	}
}
