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
	{
		upload.POST("/receipt", uploadCtrl.UploadReceipt)
	}
}
