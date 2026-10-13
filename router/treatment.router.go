package router

import (
	"github.com/Nina-99/yotuvia-api/controller"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

// TreatmentRoutes mounts the public global treatment catalog. It is read-only
// and requires no authentication: it is the shared vocabulary the search UI and
// the center's service editor consume.
func TreatmentRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewTreatmentRepositoryImpl(db)
	svc := service.NewTreatmentServiceImpl(repo)
	ctrl := controller.NewTreatmentController(svc)

	rg.GET("/treatments", ctrl.Catalog)
}
