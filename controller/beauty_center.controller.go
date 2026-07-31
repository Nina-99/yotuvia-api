package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type BeautyCenterController struct {
	beautyCenterService service.BeautyCenterService
}

func NewBeautyCenterController(service service.BeautyCenterService) *BeautyCenterController {
	return &BeautyCenterController{beautyCenterService: service}
}

func (c *BeautyCenterController) Update(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var req request.UpdateBeautyCenterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := c.beautyCenterService.Update(uint(id), req)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.HasPrefix(err.Error(), "Key:") {
			code = http.StatusBadRequest
		}
		ctx.JSON(code, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, res)
}

func (c *BeautyCenterController) Delete(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	if err := c.beautyCenterService.Delete(uint(id)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Beauty center deleted successfully"})
}

func (c *BeautyCenterController) FindByID(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	res, err := c.beautyCenterService.FindByID(uint(id))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Beauty center not found"})
		return
	}

	ctx.JSON(http.StatusOK, res)
}

func (c *BeautyCenterController) FindAll(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	res, total, err := c.beautyCenterService.FindAll(page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response.PaginatedResponse{
		Data:  res,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

func (c *BeautyCenterController) GetAvailableSlots(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	date := ctx.Query("date")
	if date == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "date query param is required (YYYY-MM-DD)"})
		return
	}

	duration, _ := strconv.Atoi(ctx.DefaultQuery("duration", "30"))
	if duration < 30 {
		duration = 30
	}

	personnelID, _ := strconv.Atoi(ctx.DefaultQuery("personnel_id", "0"))

	res, err := c.beautyCenterService.GetAvailableSlots(uint(id), date, duration, uint(personnelID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// Search implements beauty center search with pagination
func (c *BeautyCenterController) Search(ctx *gin.Context) {
	query := ctx.Query("q")
	if query == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "query param q is required"})
		return
	}

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	res, total, err := c.beautyCenterService.Search(query, page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response.PaginatedResponse{
		Data:  res,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// FindTopRated returns top-rated beauty centers
func (c *BeautyCenterController) FindTopRated(ctx *gin.Context) {
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "10"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	res, err := c.beautyCenterService.FindTopRated(limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// FindNearby returns beauty centers sorted by distance
func (c *BeautyCenterController) FindNearby(ctx *gin.Context) {
	lat, err := strconv.ParseFloat(ctx.Query("lat"), 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "lat query param is required (float)"})
		return
	}

	lng, err := strconv.ParseFloat(ctx.Query("lng"), 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "lng query param is required (float)"})
		return
	}

	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "10"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	res, err := c.beautyCenterService.FindNearby(lat, lng, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, res)
}
