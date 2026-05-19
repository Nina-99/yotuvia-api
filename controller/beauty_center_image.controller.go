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
