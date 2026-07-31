package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type ReviewController struct {
	service service.ReviewService
}

func NewReviewController(service service.ReviewService) *ReviewController {
	return &ReviewController{service: service}
}

func (c *ReviewController) Create(ctx *gin.Context) {
	var req request.CreateReviewRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	clientID, exists := ctx.Get("clientID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := c.service.Create(clientID.(uint), req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "review created successfully"})
}

func (c *ReviewController) Update(ctx *gin.Context) {
	reviewIDStr := ctx.Param("id")
	reviewID, err := strconv.ParseUint(reviewIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid review id"})
		return
	}

	var req request.UpdateReviewRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	clientID, exists := ctx.Get("clientID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := c.service.Update(clientID.(uint), uint(reviewID), req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "review updated successfully"})
}

func (c *ReviewController) Delete(ctx *gin.Context) {
	reviewIDStr := ctx.Param("id")
	reviewID, err := strconv.ParseUint(reviewIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid review id"})
		return
	}

	clientID, exists := ctx.Get("clientID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := c.service.Delete(clientID.(uint), uint(reviewID)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "review deleted successfully"})
}

func (c *ReviewController) CanReview(ctx *gin.Context) {
	bcIDStr := ctx.Param("beautyCenterID")
	bcID, err := strconv.ParseUint(bcIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid beauty center id"})
		return
	}

	clientID, exists := ctx.Get("clientID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	canReview, err := c.service.CanReview(clientID.(uint), uint(bcID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"can_review": canReview})
}

func (c *ReviewController) FindByID(ctx *gin.Context) {
	reviewIDStr := ctx.Param("id")
	reviewID, err := strconv.ParseUint(reviewIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid review id"})
		return
	}

	review, err := c.service.FindByID(uint(reviewID))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "review not found"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": review})
}

func (c *ReviewController) FindByBeautyCenter(ctx *gin.Context) {
	bcIDStr := ctx.Param("beautyCenterID")
	bcID, err := strconv.ParseUint(bcIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid beauty center id"})
		return
	}

	reviews, err := c.service.FindByBeautyCenterID(uint(bcID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": reviews})
}

func (c *ReviewController) FindByClient(ctx *gin.Context) {
	clientIDStr := ctx.Param("clientID")
	clientID, err := strconv.ParseUint(clientIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid client id"})
		return
	}

	reviews, err := c.service.FindByClientID(uint(clientID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": reviews})
}
