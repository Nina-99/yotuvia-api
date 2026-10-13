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

type SocialMediaController struct {
	socialMediaService service.SocialMediaService
}

func NewSocialMediaController(service service.SocialMediaService) *SocialMediaController {
	return &SocialMediaController{socialMediaService: service}
}

// writeError maps the ownership sentinels through apperrors.MapToHTTP so a
// foreign centre gets 403 {"error":"forbidden"} and a missing row 404. Any
// other error keeps the pre-existing 500 contract for this controller.
func (c *SocialMediaController) writeError(ctx *gin.Context, err error) {
	if errors.Is(err, apperrors.ErrForbidden) || errors.Is(err, apperrors.ErrNotFound) {
		status, msg := apperrors.MapToHTTP(err)
		ctx.JSON(status, gin.H{"error": msg})
		return
	}
	ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

func (c *SocialMediaController) Create(ctx *gin.Context) {
	var req request.CreateSocialMediaRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The links' owner is the caller's centre from the token, never the body:
	// a center-A admin must not create center-B links by sending
	// `beauty_center_id`. Same contract as loyalty/promotion creation. A
	// beauty_center token with no centerID claim (0) cannot own links, so it is
	// rejected rather than persisting a zero-centre orphan.
	centerID := ctx.GetUint("centerID")
	if centerID == 0 {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	req.BeautyCenterID = centerID

	if err := c.socialMediaService.Create(req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Social media links created successfully"})
}

func (c *SocialMediaController) Update(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var req request.UpdateSocialMediaRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The service enforces row.BeautyCenterID == the caller's centerID claim.
	if err := c.socialMediaService.Update(uint(id), ctx.GetUint("centerID"), req); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Social media links updated successfully"})
}

func (c *SocialMediaController) Delete(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	// The service enforces row.BeautyCenterID == the caller's centerID claim.
	if err := c.socialMediaService.Delete(uint(id), ctx.GetUint("centerID")); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusNoContent, nil)
}

func (c *SocialMediaController) FindByID(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	res, err := c.socialMediaService.FindByID(uint(id))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, res)
}

func (c *SocialMediaController) FindByBeautyCenterID(ctx *gin.Context) {
	centerIdParam := ctx.Param("id")
	centerId, err := strconv.Atoi(centerIdParam)
	if err != nil || centerId <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Beauty Center ID format"})
		return
	}

	res, err := c.socialMediaService.FindByBeautyCenterID(uint(centerId))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, res)
}
