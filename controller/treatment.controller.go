package controller

import (
	"net/http"

	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

// TreatmentController exposes the public global treatment catalog.
type TreatmentController struct {
	treatmentService service.TreatmentService
}

func NewTreatmentController(svc service.TreatmentService) *TreatmentController {
	return &TreatmentController{treatmentService: svc}
}

// Catalog handles GET /treatments. Public: the vocabulary is the same for every
// visitor and is required before authentication to power the search UI.
func (c *TreatmentController) Catalog(ctx *gin.Context) {
	res, err := c.treatmentService.Catalog()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, res)
}
