package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type LoyaltyController struct {
	loyaltyService service.LoyaltyService
}

func NewLoyaltyController(service service.LoyaltyService) *LoyaltyController {
	return &LoyaltyController{loyaltyService: service}
}

// CreateProgram creates a new loyalty program for a beauty center.
func (c *LoyaltyController) CreateProgram(ctx *gin.Context) {
	var req request.CreateLoyaltyProgrmaRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := c.loyaltyService.CreateProgram(req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, res)
}

// GetCLientCards returns all loyalty cards for a specific customer.
func (c *LoyaltyController) GetCLientCards(ctx *gin.Context) {
	clientIdParam := ctx.Param("clientId")
	clientId, err := strconv.Atoi(clientIdParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format of ID invalid"})
		return
	}

	res, err := c.loyaltyService.GetClientsCards(uint(clientId))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// FindByCenter returns all loyalty programs for a beauty center.
func (c *LoyaltyController) FindByCenter(ctx *gin.Context) {
	centerIDParam := ctx.Param("centerId")
	centerID, err := strconv.Atoi(centerIDParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid center ID"})
		return
	}

	res, err := c.loyaltyService.FindProgramsByCenter(uint(centerID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// UpdateProgram updates an existing loyalty program.
func (c *LoyaltyController) UpdateProgram(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var req request.UpdateLoyaltyProgramRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := c.loyaltyService.UpdateProgram(uint(id), req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// DeleteProgram soft-deletes a loyalty program.
func (c *LoyaltyController) DeleteProgram(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := c.loyaltyService.DeleteProgram(uint(id)); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusNoContent, nil)
}

// ToggleProgram enables or disables a loyalty program.
func (c *LoyaltyController) ToggleProgram(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var body struct {
		IsActive bool `json:"is_active"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.loyaltyService.ToggleProgram(uint(id), body.IsActive); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"is_active": body.IsActive})
}

// GetCenterSettings returns the loyalty settings for a beauty center.
func (c *LoyaltyController) GetCenterSettings(ctx *gin.Context) {
	centerIDParam := ctx.Param("centerId")
	centerID, err := strconv.Atoi(centerIDParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid center ID"})
		return
	}

	res, err := c.loyaltyService.GetCenterSettings(uint(centerID))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// UpdateCenterSettings updates the loyalty settings for a beauty center.
func (c *LoyaltyController) UpdateCenterSettings(ctx *gin.Context) {
	centerIDParam := ctx.Param("centerId")
	centerID, err := strconv.Atoi(centerIDParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid center ID"})
		return
	}

	var req request.UpdateLoyaltySettingsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.loyaltyService.UpdateCenterSettings(uint(centerID), req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "settings updated"})
}
