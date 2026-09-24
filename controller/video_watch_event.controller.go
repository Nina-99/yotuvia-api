package controller

import (
	"net/http"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type VideoWatchEventController struct {
	watchEventService service.VideoWatchEventService
	feedPostService   service.FeedPostService
}

func NewVideoWatchEventController(watchSvc service.VideoWatchEventService, feedSvc service.FeedPostService) *VideoWatchEventController {
	return &VideoWatchEventController{
		watchEventService: watchSvc,
		feedPostService:   feedSvc,
	}
}

// RecordWatch handles POST /video-watch-events
func (c *VideoWatchEventController) RecordWatch(ctx *gin.Context) {
	var req request.RecordWatchRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	clientID, exists := ctx.Get("clientID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if err := c.watchEventService.RecordWatch(clientID.(uint), req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "watch recorded"})
}

// GetPersonalizedFeed handles GET /feed-posts/personalized
func (c *VideoWatchEventController) GetPersonalizedFeed(ctx *gin.Context) {
	clientID, exists := ctx.Get("clientID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get all posts to match against personalized IDs (with clientID for is_liked/is_saved)
	clientIDUint := clientID.(uint)
	allPosts, _, err := c.feedPostService.FindAll(1, 1000, &clientIDUint)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build lookup map
	postMap := make(map[uint]response.FeedPostResponse, len(allPosts))
	for _, p := range allPosts {
		postMap[p.ID] = p
	}

	// Get personalized IDs
	ids, err := c.watchEventService.GetPersonalizedFeed(clientIDUint, 50)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Collect posts in ranked order
	var posts []response.FeedPostResponse
	for _, id := range ids {
		if p, ok := postMap[id]; ok {
			posts = append(posts, p)
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"posts": posts, "total": len(posts)})
}
