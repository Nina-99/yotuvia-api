package router

import (
	"github.com/Nina-99/yotuvia-api/controller"
	"github.com/Nina-99/yotuvia-api/middleware"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

// TikTokRoutes wires the "connect your TikTok account" flow. The integration is
// inert unless TIKTOK_CLIENT_KEY/SECRET/REDIRECT_URI and TIKTOK_TOKEN_KEY are
// set (the service reports Enabled and the endpoints answer 503 otherwise).
func TikTokRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	// The encryption key is optional at wire time: a missing/invalid key simply
	// keeps the service disabled rather than failing boot.
	key, err := utils.TokenEncryptionKey()
	if err != nil {
		utils.Log.Warnf("[TIKTOK] integration disabled: %v", err)
		key = nil
	}

	svc := service.NewTikTokServiceImpl(
		repository.NewTikTokAccountRepositoryImpl(db),
		service.NewHTTPTikTokClient(nil),
		NewFeedPostService(db, validate),
		service.TikTokConfigFromEnv(),
		key,
	)
	ctrl := controller.NewTikTokController(svc)

	group := rg.Group("/integrations/tiktok")
	{
		// Public: TikTok redirects the user's browser here after consent.
		group.GET("/callback", ctrl.Callback)

		protected := group.Group("")
		protected.Use(middleware.JWTAuthMiddleware(), middleware.RequireRole("beauty_center"))
		{
			protected.GET("/connect", ctrl.Connect)
			protected.GET("/account", ctrl.Account)
			protected.GET("/videos", ctrl.Videos)
			protected.POST("/import", ctrl.Import)
			protected.DELETE("/account", ctrl.Disconnect)
		}
	}
}
