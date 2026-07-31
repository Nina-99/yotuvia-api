package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type ClientController struct {
	clientService service.ClientService
}

func NewClientController(service service.ClientService) *ClientController {
	return &ClientController{clientService: service}
}

func (c *ClientController) Update(ctx *gin.Context) {
	idParam := ctx.Param("id")
	clientId, err := strconv.Atoi(idParam)
	if err != nil || clientId <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format of ID invalid"})
		return
	}
	var req request.UpdateClientRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.clientService.Update(uint(clientId), req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Update"})
}

func (c *ClientController) Delete(ctx *gin.Context) {
	idParam := ctx.Param("id")
	clientId, err := strconv.Atoi(idParam)
	if err != nil || clientId <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format of ID invalid"})
		return
	}
	if err := c.clientService.Delete(uint(clientId)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

func (c *ClientController) FindAll(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	clients, total, err := c.clientService.FindAll(page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var res []response.ClientResponse
	for _, client := range clients {
		res = append(res, response.ClientResponse{ID: client.ID, FirstName: client.FirstName, LastName: client.LastName, Username: client.Username, Email: client.Email})
	}
	ctx.JSON(http.StatusOK, response.PaginatedResponse{
		Data:  res,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

func (c *ClientController) GetUserByEmail(ctx *gin.Context) {
	Email := ctx.Param("email")
	client, err := c.clientService.FindEmail(Email)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var res []response.ClientResponse
	res = append(res, response.ClientResponse{ID: client.ID, FirstName: client.FirstName, LastName: client.LastName, Username: client.Username, Email: client.Email})
	ctx.JSON(http.StatusOK, res)
}

func (c *ClientController) FindMe(ctx *gin.Context) {
	clientID := ctx.GetUint("clientID")
	if clientID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "client ID not found in token"})
		return
	}

	client, err := c.clientService.FindByID(clientID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Client not found"})
		return
	}

	ctx.JSON(http.StatusOK, response.ClientResponse{
		ID:        client.ID,
		FirstName: client.FirstName,
		LastName:  client.LastName,
		Username:  client.Username,
		Email:     client.Email,
		Phone:     client.Phone,
		AvatarURL: client.AvatarURL,
		IsActive:  client.IsActive,
	})
}

func (c *ClientController) FindByID(ctx *gin.Context) {
	idParam := ctx.Param("id")
	clientID, err := strconv.Atoi(idParam)
	if err != nil || clientID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format of ID invalid"})
		return
	}

	client, err := c.clientService.FindByID(uint(clientID))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Client not found"})
		return
	}

	ctx.JSON(http.StatusOK, response.ClientResponse{
		ID:        client.ID,
		FirstName: client.FirstName,
		LastName:  client.LastName,
		Username:  client.Username,
		Email:     client.Email,
		Phone:     client.Phone,
		AvatarURL: client.AvatarURL,
		IsActive:  client.IsActive,
	})
}
