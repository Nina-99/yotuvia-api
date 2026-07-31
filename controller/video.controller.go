package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type VideoController struct {
	videoService service.VideoService
}

func NewVideoController(svc service.VideoService) *VideoController {
	return &VideoController{videoService: svc}
}

func (c *VideoController) Upload(ctx *gin.Context) {
	centerID := ctx.GetUint("centerID")
	if centerID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Only beauty centers can upload videos"})
		return
	}

	title := ctx.PostForm("title")
	description := ctx.PostForm("description")
	file, err := ctx.FormFile("video")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "video file is required"})
		return
	}

	video, err := c.videoService.Upload(centerID, title, description, file)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"id":          video.ID,
		"title":       video.Title,
		"file_name":   video.FileName,
		"description": video.Description,
	})
}

func (c *VideoController) Delete(ctx *gin.Context) {
	centerID := ctx.GetUint("centerID")
	idParam := ctx.Param("id")
	videoID, err := strconv.Atoi(idParam)
	if err != nil || videoID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := c.videoService.Delete(uint(videoID), centerID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

func (c *VideoController) Feed(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	videos, total, err := c.videoService.GetFeed(page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response.PaginatedResponse{
		Data:  mapVideosResponse(videos),
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

func (c *VideoController) GetByCenter(ctx *gin.Context) {
	centerIDStr := ctx.Param("centerId")
	centerID, err := strconv.Atoi(centerIDStr)
	if err != nil || centerID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid center ID"})
		return
	}

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	videos, total, err := c.videoService.GetByCenter(uint(centerID), page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response.PaginatedResponse{
		Data:  mapVideosResponse(videos),
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

func (c *VideoController) IncrementViews(ctx *gin.Context) {
	idParam := ctx.Param("id")
	videoID, err := strconv.Atoi(idParam)
	if err != nil || videoID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := c.videoService.IncrementViews(uint(videoID)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "OK"})
}

// ─── Helpers ─────────────────────────────────

type VideoResponse struct {
	ID             uint   `json:"id"`
	BeautyCenterID uint   `json:"beauty_center_id"`
	CenterName     string `json:"center_name"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	FileName       string `json:"file_name"`
	VideoURL       string `json:"video_url"`
	Views          int    `json:"views"`
	CreatedAt      string `json:"created_at"`
}

func mapVideosResponse(videos []models.Video) []VideoResponse {
	result := make([]VideoResponse, len(videos))
	for i, v := range videos {
		result[i] = VideoResponse{
			ID:             v.ID,
			BeautyCenterID: v.BeautyCenterID,
			CenterName:     v.BeautyCenter.Name,
			Title:          v.Title,
			Description:    v.Description,
			FileName:       v.FileName,
			VideoURL:       "/api/videos/stream/" + v.FileName,
			Views:          v.Views,
			CreatedAt:      v.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	return result
}
