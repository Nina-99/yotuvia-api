package router

import (
	"github.com/Nina-99/yotuvia-api/controller"
	"github.com/Nina-99/yotuvia-api/middleware"
	"github.com/gin-gonic/gin"
)

func UploadRoutes(rg *gin.RouterGroup) {
	uploadCtrl := controller.NewUploadController()

	upload := rg.Group("/upload")
	upload.Use(middleware.JWTAuthMiddleware(), middleware.RequireRole("beauty_center", "client"))
	// Tight per-IP budget + free-space floor: uploads are the only endpoints
	// that write to the volume Postgres also lives on.
	upload.Use(middleware.UploadGuard()...)
	{
		upload.POST("/receipt", uploadCtrl.UploadReceipt)
	}
}
