package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type PromotionController struct {
	promoService service.PromotionService
}

func NewPromotionController(svc service.PromotionService) *PromotionController {
	return &PromotionController{promoService: svc}
}

// Create creates a new promotion for a beauty center.
func (c *PromotionController) Create(ctx *gin.Context) {
	var req request.CreatePromotionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := c.promoService.Create(req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, res)
}

// FindByID returns a single promotion by ID.
func (c *PromotionController) FindByID(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	res, err := c.promoService.FindByID(uint(id))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, res)
}

// FindByCenter returns all promotions for a beauty center.
func (c *PromotionController) FindByCenter(ctx *gin.Context) {
	centerIDParam := ctx.Param("centerId")
	centerID, err := strconv.Atoi(centerIDParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid center ID"})
		return
	}

	includeInactive := ctx.Query("include_inactive") == "true"
	res, err := c.promoService.FindByCenter(uint(centerID), includeInactive)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, res)
}

// Update modifies an existing promotion.
func (c *PromotionController) Update(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var req request.UpdatePromotionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := c.promoService.Update(uint(id), req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, res)
}

// Delete soft-deletes a promotion.
func (c *PromotionController) Delete(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := c.promoService.Delete(uint(id)); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusNoContent, nil)
}

// FindApplicable returns promotions applicable to a specific service at the current time.
func (c *PromotionController) FindApplicable(ctx *gin.Context) {
	centerIDParam := ctx.Param("centerId")
	centerID, err := strconv.Atoi(centerIDParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid center ID"})
		return
	}

	serviceIDParam := ctx.Query("service_id")
	serviceID, err := strconv.Atoi(serviceIDParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	hour := 0
	dayOfWeek := 0
	if h := ctx.Query("hour"); h != "" {
		hour, _ = strconv.Atoi(h)
	}
	if d := ctx.Query("day_of_week"); d != "" {
		dayOfWeek, _ = strconv.Atoi(d)
	}

	res, err := c.promoService.FindApplicable(uint(centerID), uint(serviceID), hour, dayOfWeek)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, res)
}
