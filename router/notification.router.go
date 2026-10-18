package router

import (
	"github.com/Nina-99/yotuvia-api/controller"
	"github.com/Nina-99/yotuvia-api/middleware"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
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

	// Personnel notifications (personnel auth, design D5). Routes are
	// claim-scoped: the actor's personnelID lives in ctx "clientID", so no
	// path param is exposed and cross-staff access is rejected by the service.
	personnel := notifications.Group("/personnel")
	personnel.Use(middleware.RequireRole("personnel"))
	{
		personnel.GET("", ctrl.GetByPersonnel)
		personnel.GET("/unread", ctrl.GetUnreadByPersonnel)
		personnel.GET("/count", ctrl.CountUnreadByPersonnel)
		personnel.PATCH("/:id/read", ctrl.MarkAsReadByPersonnel)
	}

	// Center notifications (beauty_center auth, design D1/D6). Routes are
	// claim-scoped: the actor's centerID lives in ctx "centerID", so no path
	// param is exposed on list/unread/count, and cross-center access is
	// rejected by the service.
	center := notifications.Group("/center")
	center.Use(middleware.RequireRole("beauty_center"))
	{
		center.GET("", ctrl.GetByCenter)
		center.GET("/unread", ctrl.GetUnreadByCenter)
		center.GET("/count", ctrl.CountUnreadByCenter)
		center.PATCH("/:id/read", ctrl.MarkAsReadByCenter)
	}

	// Push devices (client auth)
	push := rg.Group("/push-devices")
	{
		// PUBLIC: the VAPID application server key must be readable before the
		// browser has a session, since subscribing is a signed-out-safe step.
		push.GET("/vapid-public-key", ctrl.VapidPublicKey)
	}
	pushAuth := rg.Group("/push-devices")
	pushAuth.Use(middleware.JWTAuthMiddleware())
	{
		pushAuth.POST("/register", ctrl.RegisterPushDevice)
		pushAuth.POST("/unregister", ctrl.UnregisterPushDevice)
	}
}
