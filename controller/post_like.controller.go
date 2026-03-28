package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type PostLikeController struct {
	postLikeService service.PostLikeService
}

func NewPostLikeController(service service.PostLikeService) *PostLikeController {
	return &PostLikeController{postLikeService: service}
}

// Create handles POST /post-likes
func (c *PostLikeController) Create(ctx *gin.Context) {
	// Extract postID from request body or param?
	// The task says POST /post-likes, but common practice is to have postID in body or URL.
	// Based on design: CreatePostLikeRequest { FeedPostID uint }
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

	like, err := c.postLikeService.Create(clientID.(uint), req.FeedPostID)
	if err != nil {
		if err.Error() == "already liked this post" {
			ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, like)
}

// Delete handles DELETE /post-likes/post/:post_id
func (c *PostLikeController) Delete(ctx *gin.Context) {
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

	err = c.postLikeService.Delete(clientID.(uint), uint(postID))
	if err != nil {
		if err.Error() == "like not found" {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Like removed"})
}

// GetLikesByPost handles GET /post-likes/post/:post_id
func (c *PostLikeController) GetLikesByPost(ctx *gin.Context) {
	postIDStr := ctx.Param("post_id")
	postID, err := strconv.ParseUint(postIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	likes, err := c.postLikeService.GetLikesByPostID(uint(postID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, likes)
}

// CheckIfUserLiked handles GET /post-likes/post/:post_id/check
func (c *PostLikeController) CheckIfUserLiked(ctx *gin.Context) {
	postIDStr := ctx.Param("post_id")
	postID, err := strconv.ParseUint(postIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	clientID, exists := ctx.Get("clientID")
	if !exists {
		// If not logged in, we can say they didn't like it or return unauthorized.
		// Usually for "check" it's better to return unauthorized if it requires auth.
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	liked, err := c.postLikeService.CheckIfUserLiked(clientID.(uint), uint(postID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"liked": liked})
}
