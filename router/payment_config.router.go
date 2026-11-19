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

func PaymentConfigRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	// Repo + Service
	paymentRepo := repository.NewPaymentConfigRepositoryImpl(db)
	paymentSvc := service.NewPaymentConfigServiceImpl(paymentRepo)
	uploadCtrl := controller.NewUploadController()
	ctrl := controller.NewPaymentConfigController(paymentSvc, uploadCtrl)

	// Public: BeautyCenters read payment config to display QR
	config := rg.Group("/config")
	{
		config.GET("/payment", ctrl.GetPaymentConfig)
		config.GET("/tiers", ctrl.GetTierConfig)
	}

	// Admin: manage payment config (QR image, phone, tier percentages)
	admin := rg.Group("/admin/config")
	admin.Use(middleware.JWTAuthMiddleware(), middleware.RequireRole("beauty_center"))
	{
		admin.PUT("/payment", ctrl.UpdatePaymentConfig)
		admin.POST("/payment/qr", middleware.WithUploadGuard(ctrl.UploadPaymentQR)...)
	}
}
