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

func FavoriteRoutes(router *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewFavoriteRepositoryImpl(db)
	svc := service.NewFavoriteServiceImpl(repo, validate)
	ctrl := controller.NewFavoriteController(svc)

	favorites := router.Group("/favorites").Use(middleware.JWTAuthMiddleware())
	{
		favorites.POST("", ctrl.Create)
		favorites.DELETE("/:id", ctrl.Delete)
	}
	clients := router.Group("/clients").Use(middleware.JWTAuthMiddleware())
	{
		clients.GET("/:client_id/favorites", ctrl.FindByClientID)
	}
}
