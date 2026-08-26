package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type VideoCommentController struct {
	videoCommentService service.VideoCommentService
}

func NewVideoCommentController(svc service.VideoCommentService) *VideoCommentController {
	return &VideoCommentController{videoCommentService: svc}
}

// CreateComment handles POST /feed-posts/:id/comments
func (c *VideoCommentController) CreateComment(ctx *gin.Context) {
	feedPostID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || feedPostID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid feed post id"})
		return
	}

	var req request.CreateCommentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := c.videoCommentService.CreateComment(feedActor(ctx), uint(feedPostID), req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, resp)
}

// ReplyToComment handles POST /feed-posts/:id/comments/:cid/reply
func (c *VideoCommentController) ReplyToComment(ctx *gin.Context) {
	commentID, err := strconv.Atoi(ctx.Param("cid"))
	if err != nil || commentID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid comment id"})
		return
	}

	var req request.ReplyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := c.videoCommentService.ReplyToComment(feedActor(ctx), uint(commentID), req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, resp)
}

// UpdateComment handles PATCH /feed-posts/:id/comments/:cid
func (c *VideoCommentController) UpdateComment(ctx *gin.Context) {
	commentID, err := strconv.Atoi(ctx.Param("cid"))
	if err != nil || commentID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid comment id"})
		return
	}

	var req request.UpdateCommentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := c.videoCommentService.UpdateComment(feedActor(ctx), uint(commentID), req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, resp)
}

// DeleteComment handles DELETE /feed-posts/:id/comments/:cid
func (c *VideoCommentController) DeleteComment(ctx *gin.Context) {
	commentID, err := strconv.Atoi(ctx.Param("cid"))
	if err != nil || commentID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid comment id"})
		return
	}

	if err := c.videoCommentService.DeleteComment(feedActor(ctx), uint(commentID)); err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "comment deleted"})
}

// HideComment handles PATCH /feed-posts/:id/comments/:cid/hide
func (c *VideoCommentController) HideComment(ctx *gin.Context) {
	commentID, err := strconv.Atoi(ctx.Param("cid"))
	if err != nil || commentID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid comment id"})
		return
	}

	if err := c.videoCommentService.HideComment(feedActor(ctx), uint(commentID)); err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "comment hidden"})
}

// ListComments handles GET /feed-posts/:id/comments
func (c *VideoCommentController) ListComments(ctx *gin.Context) {
	feedPostID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || feedPostID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid feed post id"})
		return
	}

	var cursor *uint
	if cs := ctx.Query("cursor"); cs != "" {
		cv, err := strconv.Atoi(cs)
		if err == nil && cv > 0 {
			v := uint(cv)
			cursor = &v
		}
	}

	comments, nextCursor, err := c.videoCommentService.ListComments(uint(feedPostID), cursor, 20)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, response.CommentListResponse{
		Comments:   comments,
		NextCursor: nextCursor,
	})
}

// GetCommentReplies handles GET /feed-posts/:id/comments/:cid
func (c *VideoCommentController) GetCommentReplies(ctx *gin.Context) {
	commentID, err := strconv.Atoi(ctx.Param("cid"))
	if err != nil || commentID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid comment id"})
		return
	}

	replies, err := c.videoCommentService.GetCommentReplies(uint(commentID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, response.CommentListResponse{Comments: replies})
}
