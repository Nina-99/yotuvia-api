package router

import (
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/middleware"
	"github.com/gin-gonic/gin"
)

func UploadRoutes(rg *gin.RouterGroup) {
	uploadCtrl := controller.NewUploadController()

	upload := rg.Group("/upload")
	upload.Use(middleware.JWTAuthMiddleware(), middleware.RequireRole("beauty_center"))
	{
		upload.POST("/receipt", uploadCtrl.UploadReceipt)
	}
}