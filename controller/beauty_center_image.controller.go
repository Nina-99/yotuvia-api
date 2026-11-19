package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
)

type BeautyCenterImageController struct {
	BeautyCenterImageService service.BeautyCenterImageService
}

func NewBeautyCenterImageController(beautyCenterImageService service.BeautyCenterImageService) *BeautyCenterImageController {
	return &BeautyCenterImageController{
		BeautyCenterImageService: beautyCenterImageService,
	}
}

// writeError maps the ownership sentinels through apperrors.MapToHTTP so a
// foreign centre gets 403 {"error":"forbidden"} and a missing row 404. Any
// other error keeps the pre-existing 400 contract for this controller.
func (c *BeautyCenterImageController) writeError(ctx *gin.Context, err error) {
	if errors.Is(err, apperrors.ErrForbidden) || errors.Is(err, apperrors.ErrNotFound) {
		status, msg := apperrors.MapToHTTP(err)
		ctx.JSON(status, gin.H{"error": msg})
		return
	}
	writeBadRequest(ctx, err)
}

func (c *BeautyCenterImageController) Upload(ctx *gin.Context) {
	centerID := ctx.GetUint("centerID")
	if centerID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Only beauty centers can upload images"})
		return
	}

	isCover := ctx.PostForm("is_cover") == "true"
	isProfile := ctx.PostForm("is_profile") == "true"

	file, err := ctx.FormFile("image")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "image file is required"})
		return
	}

	res, err := c.BeautyCenterImageService.Upload(centerID, file, isCover, isProfile)
	if err != nil {
		writeInternalError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, res)
}

func (c *BeautyCenterImageController) Create(ctx *gin.Context) {
	var req request.BeautyCenterImageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// The image's owner is the caller's centre from the token, never the body:
	// a center-A admin must not add a center-B image by sending
	// `beauty_center_id`. Same contract as loyalty/promotion creation. A
	// beauty_center token with no centerID claim (0) cannot own an image, so it
	// is rejected rather than persisting a zero-centre orphan.
	centerID := ctx.GetUint("centerID")
	if centerID == 0 {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	req.BeautyCenterID = centerID

	res, err := c.BeautyCenterImageService.Create(req)
	if err != nil {
		writeBadRequest(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, res)
}

func (c *BeautyCenterImageController) Update(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id format"})
		return
	}

	var req request.UpdateBeautyCenterImageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// The service enforces row.BeautyCenterID == the caller's centerID claim.
	res, err := c.BeautyCenterImageService.Update(uint(id), ctx.GetUint("centerID"), req)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, res)
}

func (c *BeautyCenterImageController) Reorder(ctx *gin.Context) {
	centerID := ctx.GetUint("centerID")
	if centerID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req request.ReorderImagesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if err := c.BeautyCenterImageService.Reorder(centerID, req); err != nil {
		writeBadRequest(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Order updated successfully"})
}

func (c *BeautyCenterImageController) FindByID(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id format"})
		return
	}

	res, err := c.BeautyCenterImageService.GetByID(uint(id))
	if err != nil {
		writeBadRequest(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, res)
}

func (c *BeautyCenterImageController) Delete(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id format"})
		return
	}

	err = c.BeautyCenterImageService.Delete(uint(id), ctx.GetUint("centerID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Beauty Center Image delete successfully"})
}

func (c *BeautyCenterImageController) FindByBeautyCenterID(ctx *gin.Context) {
	beautyCenterIDParam := ctx.Param("id")
	beautyCenterID, err := strconv.ParseUint(beautyCenterIDParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id format"})
		return
	}

	res, err := c.BeautyCenterImageService.GetByBeautyCenterID(uint(beautyCenterID))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, res)
}
