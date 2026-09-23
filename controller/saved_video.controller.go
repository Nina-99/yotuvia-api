package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type SavedVideoController struct {
	savedVideoService service.SavedVideoService
}

func NewSavedVideoController(svc service.SavedVideoService) *SavedVideoController {
	return &SavedVideoController{savedVideoService: svc}
}

// Toggle handles POST /saved-videos/toggle
func (c *SavedVideoController) Toggle(ctx *gin.Context) {
	var req struct {
		FeedPostID uint `json:"feed_post_id" binding:"required"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	clientID, exists := ctx.Get("clientID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	saved, err := c.savedVideoService.Toggle(clientID.(uint), req.FeedPostID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"saved": saved})
}

// CheckIfSaved handles GET /saved-videos/post/:post_id/check
func (c *SavedVideoController) CheckIfSaved(ctx *gin.Context) {
	postIDStr := ctx.Param("post_id")
	postID, err := strconv.ParseUint(postIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	clientID, exists := ctx.Get("clientID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	saved, err := c.savedVideoService.CheckIfSaved(clientID.(uint), uint(postID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"saved": saved})
}

// ListByClient handles GET /saved-videos
func (c *SavedVideoController) ListByClient(ctx *gin.Context) {
	clientID, exists := ctx.Get("clientID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	savedVideos, err := c.savedVideoService.ListByClient(clientID.(uint))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, savedVideos)
}
