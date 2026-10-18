package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
)

// TimeBlockController exposes unavailability blocks (breaks, holidays) for a
// professional or the whole centre.
type TimeBlockController struct {
	timeBlockService service.TimeBlockService
}

func NewTimeBlockController(svc service.TimeBlockService) *TimeBlockController {
	return &TimeBlockController{timeBlockService: svc}
}

func (c *TimeBlockController) writeError(ctx *gin.Context, err error) {
	status, msg := apperrors.MapToHTTP(err)
	ctx.JSON(status, gin.H{"error": msg})
}

// Create handles POST /time-blocks. The actor (role + id) and the centre come
// from the JWT; the body never names the centre.
func (c *TimeBlockController) Create(ctx *gin.Context) {
	var req request.CreateTimeBlockRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	block, err := c.timeBlockService.Create(
		req,
		ctx.GetString("role"),
		ctx.GetUint("clientID"),
		ctx.GetUint("centerID"),
	)
	if err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, block)
}

// List handles GET /time-blocks?from=&to=&personnel_id=.
func (c *TimeBlockController) List(ctx *gin.Context) {
	var personnelID *uint
	if raw := ctx.Query("personnel_id"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			id := uint(value)
			personnelID = &id
		}
	}

	blocks, err := c.timeBlockService.List(
		ctx.GetString("role"),
		ctx.GetUint("clientID"),
		ctx.GetUint("centerID"),
		ctx.Query("from"),
		ctx.Query("to"),
		personnelID,
	)
	if err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, blocks)
}

// Delete handles DELETE /time-blocks/:id.
func (c *TimeBlockController) Delete(ctx *gin.Context) {
	id, ok := parsePositiveID(ctx.Param("id"))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}
	if err := c.timeBlockService.Delete(
		id,
		ctx.GetString("role"),
		ctx.GetUint("clientID"),
		ctx.GetUint("centerID"),
	); err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Time block deleted"})
}
