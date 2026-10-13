package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
)

// BoostController exposes the center-facing CPM promotion surface.
type BoostController struct {
	boostService service.BoostService
}

func NewBoostController(boostService service.BoostService) *BoostController {
	return &BoostController{boostService: boostService}
}

// Create handles POST /boost-campaigns (beauty_center). It holds the budget up
// front via the token account service and starts an ACTIVE campaign.
func (c *BoostController) Create(ctx *gin.Context) {
	var req request.CreateBoostCampaignRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.EndsAt.IsZero() || !req.EndsAt.After(time.Now()) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "ends_at must be in the future"})
		return
	}
	startsAt := req.StartsAt
	if startsAt.IsZero() {
		startsAt = time.Now()
	}

	campaign, err := c.boostService.CreateCampaign(
		ctx.GetUint("centerID"),
		req.FeedPostID,
		utils.ToMilli(req.Budget),
		utils.ToMilli(req.BidCPM),
		startsAt,
		req.EndsAt,
	)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, campaign)
}

// List handles GET /boost-campaigns (beauty_center): the center's own campaigns.
func (c *BoostController) List(ctx *gin.Context) {
	campaigns, err := c.boostService.ListByCenter(ctx.GetUint("centerID"))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"campaigns": campaigns})
}

// Pause handles PUT /boost-campaigns/:id/pause (beauty_center, owner only).
func (c *BoostController) Pause(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}
	if err := c.boostService.PauseCampaign(ctx.GetUint("centerID"), uint(id)); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "campaign paused"})
}
