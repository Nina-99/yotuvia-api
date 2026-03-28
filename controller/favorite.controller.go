package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type FavoriteController struct {
	FavoriteService service.FavoriteService
}

func NewFavoriteController(service service.FavoriteService) *FavoriteController {
	return &FavoriteController{FavoriteService: service}
}

func (c *FavoriteController) Create(ctx *gin.Context) {
	var req request.CreateFavoriteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Retrieve ClientID from context (set by JWT middleware)
	clientID, exists := ctx.Get("clientID")
	if exists && clientID != nil {
		req.ClientID = clientID.(uint)
	}

	if err := c.FavoriteService.Create(req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{"message": "favorite added successfully"})
}

func (c *FavoriteController) Delete(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := c.FavoriteService.Delete(uint(id)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "favorite removed successfully"})
}

func (c *FavoriteController) FindByClientID(ctx *gin.Context) {
	clientIDParam := ctx.Param("client_id")
	clientID, err := strconv.ParseUint(clientIDParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid client_id"})
		return
	}

	favorites, err := c.FavoriteService.FindByClientID(uint(clientID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, favorites)
}
