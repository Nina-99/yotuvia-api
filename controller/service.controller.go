package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type ServiceController struct {
	serviceService service.ServiceService
}

func NewServiceController(service service.ServiceService) *ServiceController {
	return &ServiceController{serviceService: service}
}

// Create godoc
// @Summary Create a new service
// @Description Create a new service with the given details
// @Tags services
// @Accept json
// @Produce json
// @Param service body request.CreateServiceRequest true "Service details"
// @Success 201 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /services [post]
func (c *ServiceController) Create(ctx *gin.Context) {
	var req request.CreateServiceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createdID, err := c.serviceService.Create(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"id": createdID})
}

// Update godoc
// @Summary Update an existing service
// @Description Update the details of an existing service
// @Tags services
// @Accept json
// @Produce json
// @Param id path int true "Service ID"
// @Param service body request.UpdateServiceRequest true "Service details"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /services/{id} [put]
func (c *ServiceController) Update(ctx *gin.Context) {
	idParam := ctx.Param("id")
	serviceID, err := strconv.Atoi(idParam)
	if err != nil || serviceID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var req request.UpdateServiceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.serviceService.Update(uint(serviceID), req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Updated"})
}

// Delete godoc
// @Summary Delete a service
// @Description Delete an existing service by ID
// @Tags services
// @Param id path int true "Service ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /services/{id} [delete]
func (c *ServiceController) Delete(ctx *gin.Context) {
	idParam := ctx.Param("id")
	serviceID, err := strconv.Atoi(idParam)
	if err != nil || serviceID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	if err := c.serviceService.Delete(uint(serviceID)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// FindAll godoc
// @Summary List all services
// @Description Get a list of all services with optional filters by beauty_center_id, category_id, and is_active
// @Tags services
// @Produce json
// @Param beauty_center_id query int false "Filter by beauty center ID"
// @Param category_id query int false "Filter by category ID"
// @Param is_active query bool false "Filter by active status"
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 20, max: 100)"
// @Success 200 {object} response.PaginatedResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /services [get]
func (c *ServiceController) FindAll(ctx *gin.Context) {
	var beautyCenterID, categoryID uint

	if beautyCenterIDParam := ctx.Query("beauty_center_id"); beautyCenterIDParam != "" {
		id, err := strconv.Atoi(beautyCenterIDParam)
		if err != nil || id <= 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid beauty_center_id"})
			return
		}
		beautyCenterID = uint(id)
	}

	if categoryIDParam := ctx.Query("category_id"); categoryIDParam != "" {
		id, err := strconv.Atoi(categoryIDParam)
		if err != nil || id <= 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid category_id"})
			return
		}
		categoryID = uint(id)
	}

	var isActive *bool
	if isActiveParam := ctx.Query("is_active"); isActiveParam != "" {
		val, err := strconv.ParseBool(isActiveParam)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid is_active, use true/false"})
			return
		}
		isActive = &val
	}

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	services, total, err := c.serviceService.FindAll(beautyCenterID, categoryID, isActive, page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var res []response.ServiceResponse
	for _, s := range services {
		res = append(res, response.ServiceResponse{
			ID:             s.ID,
			BeautyCenterID: s.BeautyCenterID,
			CategoryID:     s.CategoryID,
			Name:           s.Name,
			Description:    s.Description,
			Duration:       s.Duration,
			Price:          s.Price,
			ImageURL:       s.ImageURL,
			IsActive:       s.IsActive,
			Category: response.CategoryResponse{
				ID:             s.Category.ID,
				BeautyCenterID: s.Category.BeautyCenterID,
				Name:           s.Category.Name,
				Description:    s.Category.Description,
			},
		})
	}

	ctx.JSON(http.StatusOK, response.PaginatedResponse{
		Data:  res,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// FindById godoc
// @Summary Get service by ID
// @Description Get details of a service by its ID
// @Tags services
// @Produce json
// @Param id path int true "Service ID"
// @Success 200 {object} response.ServiceResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /services/{id} [get]
func (c *ServiceController) FindById(ctx *gin.Context) {
	idParam := ctx.Param("id")
	serviceID, err := strconv.Atoi(idParam)
	if err != nil || serviceID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	s, err := c.serviceService.FindById(uint(serviceID))
	if err != nil || s == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	res := response.ServiceResponse{
		ID:             s.ID,
		BeautyCenterID: s.BeautyCenterID,
		CategoryID:     s.CategoryID,
		Name:           s.Name,
		Description:    s.Description,
		Duration:       s.Duration,
		Price:          s.Price,
		ImageURL:       s.ImageURL,
		IsActive:       s.IsActive,
		Category: response.CategoryResponse{
			ID:             s.Category.ID,
			BeautyCenterID: s.Category.BeautyCenterID,
			Name:           s.Category.Name,
			Description:    s.Category.Description,
		},
	}

	ctx.JSON(http.StatusOK, res)
}

// Templates godoc
// @Summary Get suggested service names by category
// @Description Returns suggested service names for a given category, in the requested language
// @Tags services
// @Produce json
// @Param category_id query int true "Category ID to get suggestions for"
// @Param lang query string false "Language: es (default) or en"
// @Success 200 {array} service.ServiceTemplate
// @Failure 400 {object} map[string]string
// @Router /services/templates [get]
func (c *ServiceController) Templates(ctx *gin.Context) {
	categoryIDParam := ctx.Query("category_id")
	if categoryIDParam == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "category_id is required"})
		return
	}

	categoryID, err := strconv.Atoi(categoryIDParam)
	if err != nil || categoryID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid category_id"})
		return
	}

	lang := ctx.DefaultQuery("lang", "es")

	suggestions, err := c.serviceService.GetServiceTemplates(uint(categoryID), lang)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if suggestions == nil {
		ctx.JSON(http.StatusOK, []service.ServiceTemplate{})
		return
	}

	ctx.JSON(http.StatusOK, suggestions)
}
