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

// CreateProgram is create a new program in the beauty center
func (c *LoyaltyController) CreateProgram(ctx *gin.Context) {
	var req request.CreateLoyaltyProgrmaRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := c.loyaltyService.CreateProgram(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, res)
}

// GetCLientCards returns all loyalty cards for a specific customer
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
