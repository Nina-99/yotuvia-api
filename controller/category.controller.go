package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/service"
	"github.com/Nina-99/yottu-api/utils"
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
	var beautyCenterID uint
	if beautyCenterIDParam := ctx.Query("beauty_center_id"); beautyCenterIDParam != "" {
		id, err := strconv.Atoi(beautyCenterIDParam)
		if err != nil || id <= 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid beauty_center_id"})
			return
		}
		beautyCenterID = uint(id)
	}

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	lang := ctx.DefaultQuery("lang", "")

	categories, total, err := c.categoryService.FindAll(beautyCenterID, page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var res []response.CategoryResponse
	for _, category := range categories {
		name := category.Name
		desc := category.Description
		icon := category.Icon

		// Fill icon from templates if not set in DB
		if icon == "" {
			icon = utils.CategoryIcon(category.Name)
		}

		// Localize name if lang is specified and name matches a known template
		if lang != "" && lang != "es" {
			localized, localizedDesc := utils.LocalizeCategoryName(category.Name, lang)
			name = localized
			if localizedDesc != "" {
				desc = localizedDesc
			}
		}

		res = append(res, response.CategoryResponse{
			ID:             category.ID,
			BeautyCenterID: category.BeautyCenterID,
			Name:           name,
			Description:    desc,
			Icon:           icon,
		})
	}
	ctx.JSON(http.StatusOK, response.PaginatedResponse{
		Data:  res,
		Total: total,
		Page:  page,
		Limit: limit,
	})
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

	lang := ctx.DefaultQuery("lang", "")
	name := s.Name
	desc := s.Description
	icon := s.Icon

	// Fill icon from templates if not set in DB
	if icon == "" {
		icon = utils.CategoryIcon(s.Name)
	}

	if lang != "" && lang != "es" {
		localized, localizedDesc := utils.LocalizeCategoryName(s.Name, lang)
		name = localized
		if localizedDesc != "" {
			desc = localizedDesc
		}
	}

	res := response.CategoryResponse{
		ID:             s.ID,
		BeautyCenterID: s.BeautyCenterID,
		Name:           name,
		Description:    desc,
		Icon:           icon,
	}

	ctx.JSON(http.StatusOK, res)
}

// Templates godoc
// @Summary Get suggested category templates
// @Description Returns suggested category names for beauty centers, in the requested language
// @Tags category
// @Produce json
// @Param lang query string false "Language: es (default) or en"
// @Success 200 {array} response.CategoryResponse
// @Router /category/templates [get]
func (c *CategoryController) Templates(ctx *gin.Context) {
	lang := ctx.DefaultQuery("lang", "es")
	templates := utils.CategoryTemplates(lang)

	var res []response.CategoryResponse
	for _, t := range templates {
		res = append(res, response.CategoryResponse{
			Name:        t.Name,
			Description: t.Description,
			Icon:        t.Icon,
		})
	}

	ctx.JSON(http.StatusOK, res)
}
