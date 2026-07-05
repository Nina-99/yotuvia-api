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

	if err := c.serviceService.Create(req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Created"})
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
// @Description Get a list of all services
// @Tags services
// @Produce json
// @Success 200 {array} response.ServiceResponse
// @Failure 500 {object} map[string]string
// @Router /services [get]
func (c *ServiceController) FindAll(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	services, total, err := c.serviceService.FindAll(page, limit)
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
