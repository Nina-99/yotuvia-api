package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/service"
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
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	res, err := c.BeautyCenterImageService.Create(req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	res, err := c.BeautyCenterImageService.Update(uint(id), req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	err = c.BeautyCenterImageService.Delete(uint(id))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
