package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
)

type BeautyCenterController struct {
	beautyCenterService service.BeautyCenterService
}

func NewBeautyCenterController(service service.BeautyCenterService) *BeautyCenterController {
	return &BeautyCenterController{beautyCenterService: service}
}

// writeError maps the ownership sentinels through apperrors.MapToHTTP so a
// foreign centre gets 403 {"error":"forbidden"} and a missing row 404. Any
// other error keeps the pre-existing contract: validator failures are 400,
// everything else is 500.
func (c *BeautyCenterController) writeError(ctx *gin.Context, err error) {
	if errors.Is(err, apperrors.ErrForbidden) || errors.Is(err, apperrors.ErrNotFound) {
		status, msg := apperrors.MapToHTTP(err)
		ctx.JSON(status, gin.H{"error": msg})
		return
	}
	code := http.StatusInternalServerError
	if strings.HasPrefix(err.Error(), "Key:") {
		code = http.StatusBadRequest
	}
	ctx.JSON(code, gin.H{"error": err.Error()})
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
		writeBadRequest(ctx, err)
		return
	}

	// The caller's centerID claim is the owner; the service enforces id == claim.
	res, err := c.beautyCenterService.Update(uint(id), ctx.GetUint("centerID"), req)
	if err != nil {
		c.writeError(ctx, err)
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

	// The caller's centerID claim is the owner; the service enforces id == claim.
	if err := c.beautyCenterService.Delete(uint(id), ctx.GetUint("centerID")); err != nil {
		c.writeError(ctx, err)
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

	res.Localize(ctx.DefaultQuery("lang", ""))
	ctx.JSON(http.StatusOK, res)
}

func (c *BeautyCenterController) FindAll(ctx *gin.Context) {
	page, limit := pagination(ctx)

	res, total, err := c.beautyCenterService.FindAll(page, limit)
	if err != nil {
		writeInternalError(ctx, err)
		return
	}

	localizeCenters(res, ctx.DefaultQuery("lang", ""))
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
		writeInternalError(ctx, err)
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

	page, limit := pagination(ctx)

	res, total, err := c.beautyCenterService.Search(query, page, limit)
	if err != nil {
		writeInternalError(ctx, err)
		return
	}

	localizeCenters(res, ctx.DefaultQuery("lang", ""))
	ctx.JSON(http.StatusOK, response.PaginatedResponse{
		Data:  res,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// FindTopRated returns top-rated beauty centers
func (c *BeautyCenterController) FindTopRated(ctx *gin.Context) {
	limit := queryLimit(ctx, 10, 50)

	res, err := c.beautyCenterService.FindTopRated(limit)
	if err != nil {
		writeInternalError(ctx, err)
		return
	}

	localizeCenters(res, ctx.DefaultQuery("lang", ""))
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

	limit := queryLimit(ctx, 10, 50)

	res, err := c.beautyCenterService.FindNearby(lat, lng, limit)
	if err != nil {
		writeInternalError(ctx, err)
		return
	}

	localizeCenters(res, ctx.DefaultQuery("lang", ""))
	ctx.JSON(http.StatusOK, res)
}

// localizeCenters resolves Name/Description for each center in place.
func localizeCenters(res []response.BeautyCenterResponse, lang string) {
	if lang == "" {
		return
	}
	for i := range res {
		res[i].Localize(lang)
	}
}

// FindRanking returns beauty centers ranked by composite score for the
// given period (week, month, or all-time).
func (c *BeautyCenterController) FindRanking(ctx *gin.Context) {
	period := ctx.DefaultQuery("period", "all")
	if period != "week" && period != "month" && period != "all" {
		period = "all"
	}

	limit := queryLimit(ctx, 10, 50)

	res, err := c.beautyCenterService.FindRanking(period, limit)
	if err != nil {
		writeInternalError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, res)
}
