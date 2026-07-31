package router

import (
	"log"

	"github.com/Nina-99/yottu-api/configs"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()
	r.Use(middleware.GinLogger())

	// Serve uploaded files
	r.Static("/uploads", "./uploads")

	validate := validator.New()
	db := configs.DB

	api := r.Group("/api")
	{
		AuthAccountRoutes(api, db, validate)
		BeautyCenterRoutes(api, db, validate)
		BeautyCenterImageRoutes(api, db, validate)
		CategoryRoutes(api, db, validate)
		ClientRoutes(api, db, validate)
		FavoriteRoutes(api, db, validate)
		FeedPostRoutes(api, db, validate)
		LoyaltyRoutes(api, db, validate)
		TokenRoutes(api, db, validate)
		PersonnelRoutes(api, db, validate)
		PersonnelInvitationRoutes(api, db, validate)
		PersonnelImageRoutes(api, db, validate)
		PersonnelServiceRoutes(api, db, validate)
		PostLikeRoutes(api, db, validate)
		ReservationRoutes(api, db, validate)
		ReviewRoutes(api, db, validate)
		NotificationRoutes(api, db)
		ServiceRoutes(api, db, validate)
		SocialMediaRoutes(api, db, validate)
		VideoRoutes(api, db, validate)
	}
	return r
}

func Run(addr string) {
	r := SetupRouter()
	log.Println("🚀 Server running on", addr)

	r.Run(addr)
}
