package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
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
		writeBadRequest(ctx, err)
		return
	}

	centerID := ctx.GetUint("centerID")
	if centerID == 0 {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	req.BeautyCenterID = centerID

	if err := c.categoryService.Create(centerID, req); err != nil {
		writeInternalError(ctx, err)
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
		writeBadRequest(ctx, err)
		return
	}

	centerID := ctx.GetUint("centerID")
	if centerID == 0 {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	if err := c.categoryService.Update(centerID, uint(categoryID), req); err != nil {
		writeInternalError(ctx, err)
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

	centerID := ctx.GetUint("centerID")
	if centerID == 0 {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	if err := c.categoryService.Delete(centerID, uint(categoryID)); err != nil {
		writeInternalError(ctx, err)
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

	page, limit := pagination(ctx)

	lang := ctx.DefaultQuery("lang", "")

	categories, total, err := c.categoryService.FindAll(beautyCenterID, page, limit)
	if err != nil {
		writeInternalError(ctx, err)
		return
	}

	var res []response.CategoryResponse
	for _, category := range categories {
		res = append(res, categoryToResponse(category, lang))
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
	res := categoryToResponse(*s, lang)

	ctx.JSON(http.StatusOK, res)
}

// categoryToResponse resolves a category's translatable fields for `lang`:
// stored per-locale overrides win, then the legacy template localization, then
// the base value. The raw maps ride along for owner edit forms.
func categoryToResponse(category models.Category, lang string) response.CategoryResponse {
	name := utils.LocalizedValue(category.Name, category.NameI18n, lang)
	desc := utils.LocalizedValue(category.Description, category.DescriptionI18n, lang)

	// Legacy fallback: no stored translation for this category, but the name is
	// a known template → localize it (existing behavior preserved).
	if category.NameI18n == nil && lang != "" && lang != "es" {
		localized, localizedDesc := utils.LocalizeCategoryName(category.Name, lang)
		name = localized
		if localizedDesc != "" {
			desc = localizedDesc
		}
	}

	icon := category.Icon
	if icon == "" {
		icon = utils.CategoryIcon(category.Name)
	}

	return response.CategoryResponse{
		ID:              category.ID,
		BeautyCenterID:  category.BeautyCenterID,
		Name:            name,
		Description:     desc,
		NameI18n:        category.NameI18n,
		DescriptionI18n: category.DescriptionI18n,
		Icon:            icon,
	}
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
