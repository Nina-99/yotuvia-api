package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NotificationRoutes(rg *gin.RouterGroup, db *gorm.DB) {
	notifRepo := repository.NewNotificationRepositoryImpl(db)
	pushRepo := repository.NewPushDeviceRepositoryImpl(db)
	svc := service.NewNotificationServiceImpl(notifRepo, pushRepo)
	ctrl := controller.NewNotificationController(svc)

	// Notifications (client auth)
	notifications := rg.Group("/notifications")
	notifications.Use(middleware.JWTAuthMiddleware())
	{
		notifications.GET("/client/:clientId", ctrl.GetByClient)
		notifications.GET("/client/:clientId/unread", ctrl.GetUnreadByClient)
		notifications.GET("/client/:clientId/count", ctrl.CountUnread)
		notifications.PATCH("/:id/read", ctrl.MarkAsRead)
		notifications.POST("/client/:clientId/read-all", ctrl.MarkAllAsRead)
	}

	// Push devices (client auth)
	push := rg.Group("/push-devices")
	push.Use(middleware.JWTAuthMiddleware())
	{
		push.POST("/register", ctrl.RegisterPushDevice)
	}
}
