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

func FavoriteRoutes(router *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewFavoriteRepositoryImpl(db)
	svc := service.NewFavoriteServiceImpl(repo, validate)
	ctrl := controller.NewFavoriteController(svc)

	// Favorites are a private, client-owned surface. DELETE and the read are
	// gated to the `client` role AND scoped to the caller's own clientID claim
	// (the service enforces ownership). Staff roles are rejected because no
	// staff/admin consumer exists — see tests/favorite_authz_test.go.
	favorites := router.Group("/favorites").Use(middleware.JWTAuthMiddleware())
	{
		favorites.POST("", ctrl.Create)
		favorites.DELETE("/:id", middleware.RequireRole("client"), ctrl.Delete)
	}
	clients := router.Group("/clients").Use(middleware.JWTAuthMiddleware())
	{
		clients.GET("/:client_id/favorites", middleware.RequireRole("client"), ctrl.FindByClientID)
	}
}
