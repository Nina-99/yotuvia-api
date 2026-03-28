package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type CategoryController struct {
	categoryService service.CategoryService
}

func NewCategoryController(service service.CategoryService) *CategoryController {
	return &CategoryController{categoryService: service}
}

func (c *CategoryController) Create(ctx *gin.Context) {
	var req request.CreateCategoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.categoryService.Create(req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Created"})
}

func (c *CategoryController) Update(ctx *gin.Context) {
	idParam := ctx.Param("id")
	categoryID, err := strconv.Atoi(idParam)
	if err != nil || categoryID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}
	var req request.CreateCategoryRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.categoryService.Update(uint(categoryID), req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Update"})
}

func (c *CategoryController) Delete(ctx *gin.Context) {
	idParam := ctx.Param("id")
	categoryID, err := strconv.Atoi(idParam)
	if err != nil || categoryID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	if err := c.categoryService.Delete(uint(categoryID)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

func (c *CategoryController) FindAll(ctx *gin.Context) {
	categorys, err := c.categoryService.FindAll()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var res []response.CategoryResponse
	for _, category := range categorys {
		res = append(res, response.CategoryResponse{ID: category.ID, BeautyCenterID: category.BeautyCenterID, Name: category.Name, Description: category.Description})
	}
	ctx.JSON(http.StatusOK, res)
}

func (c *CategoryController) FindById(ctx *gin.Context) {
	idParam := ctx.Param("id")
	categoryID, err := strconv.Atoi(idParam)
	if err != nil || categoryID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	s, err := c.categoryService.FindById(uint(categoryID))
	if err != nil || s == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	res := response.CategoryResponse{
		ID:             s.ID,
		BeautyCenterID: s.BeautyCenterID,
		Name:           s.Name,
		Description:    s.Description,
	}

	ctx.JSON(http.StatusOK, res)
}
