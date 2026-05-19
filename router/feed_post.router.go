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
	svc := service.NewFeedPostServiceImpl(repo, repoBC, validate)
	ctrl := controller.NewFeedPostController(svc)

	posts := rg.Group("/feed-posts")
	{
		posts.GET("", ctrl.FindAll)
		posts.GET("/:id", ctrl.FindByID)

		posts.Use(middleware.JWTAuthMiddleware())
		{
			posts.POST("", ctrl.Create)
			posts.PUT("/:id", ctrl.Update)
			posts.DELETE("/:id", ctrl.Delete)
			posts.GET("/center/:id", ctrl.FindByBeautyCenterID)
		}
	}
}
