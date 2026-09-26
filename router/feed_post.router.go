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

func FeedPostRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewFeedPostRepositoryImpl(db)
	repoBC := repository.NewBeautyCenterRepositoryImpl(db)
	repoPersonnel := repository.NewPersonnelRepositoryImpl(db)
	repoService := repository.NewServiceRepositoryImpl(db)
	repoLike := repository.NewPostLikeRepositoryImpl(db)
	repoSaved := repository.NewSavedVideoRepositoryImpl(db)
	repoComment := repository.NewVideoCommentRepositoryImpl(db)
	followRepo := repository.NewPersonnelFollowRepositoryImpl(db)
	notifSvc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)
	svc := service.NewFeedPostServiceImpl(repo, repoBC, repoPersonnel, repoService, repoLike, repoSaved, repoComment, followRepo, notifSvc, validate)
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
