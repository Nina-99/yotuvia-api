package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
)

type PromotionController struct {
	promoService service.PromotionService
}

func NewPromotionController(svc service.PromotionService) *PromotionController {
	return &PromotionController{promoService: svc}
}

// writeError maps the ownership sentinels through apperrors.MapToHTTP so a
// center mismatch gets 403 {"error":"forbidden"} and a missing row 404. Every
// other error keeps the previous promo 400 contract, so input-validation
// feedback is unchanged.
func (c *PromotionController) writeError(ctx *gin.Context, err error) {
	if errors.Is(err, apperrors.ErrForbidden) || errors.Is(err, apperrors.ErrNotFound) {
		status, msg := apperrors.MapToHTTP(err)
		ctx.JSON(status, gin.H{"error": msg})
		return
	}
	ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

// Create creates a new promotion for a beauty center.
func (c *PromotionController) Create(ctx *gin.Context) {
	var req request.CreatePromotionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The promo's owner is the caller's center from the token, never the body:
	// a center-A admin must not create a center-B promo by sending
	// `beauty_center_id`. A beauty_center token with no centerID claim (0)
	// cannot own a promo, so it is rejected rather than persisting a
	// zero-center orphan.
	centerID := ctx.GetUint("centerID")
	if centerID == 0 {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	req.BeautyCenterID = centerID

	res, err := c.promoService.Create(req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, res)
}

// FindByID returns a single promotion by ID, scoped to the caller's center.
func (c *PromotionController) FindByID(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	res, err := c.promoService.FindByID(uint(id), ctx.GetUint("centerID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, res)
}

// FindByCenter returns all promotions for a beauty center.
func (c *PromotionController) FindByCenter(ctx *gin.Context) {
	pathCenterID, err := strconv.Atoi(ctx.Param("centerId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid center ID"})
		return
	}

	includeInactive := ctx.Query("include_inactive") == "true"
	res, err := c.promoService.FindByCenter(uint(pathCenterID), ctx.GetUint("centerID"), includeInactive)
	if err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, res)
}

// Update modifies an existing promotion owned by the caller's center.
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

	res, err := c.promoService.Update(uint(id), ctx.GetUint("centerID"), req)
	if err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, res)
}

// Delete soft-deletes a promotion owned by the caller's center.
func (c *PromotionController) Delete(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := c.promoService.Delete(uint(id), ctx.GetUint("centerID")); err != nil {
		c.writeError(ctx, err)
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
