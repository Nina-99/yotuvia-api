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

func (c *ClientController) Login(ctx *gin.Context) {
	var req request.LoginClientRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, err := c.clientService.Login(req)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Credentials"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"token": token})
}

func (c *ClientController) Register(ctx *gin.Context) {
	var req request.RegisterClientRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, err := c.clientService.Register(req)
	if err != nil {
		if err.Error() == "email already registered" {
			ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"token": token})
}

func (c *ClientController) Update(ctx *gin.Context) {
	idParam := ctx.Param("id")
	clientId, err := strconv.Atoi(idParam)
	if err != nil || clientId <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format of ID invalid"})
		return
	}
	var req request.RegisterClientRequest

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
	clients, err := c.clientService.FindAll()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var res []response.ClientResponse
	for _, client := range clients {
		res = append(res, response.ClientResponse{ID: client.ID, FirstName: client.FirstName, LastName: client.LastName, Username: client.Username, Email: client.Email})
	}
	ctx.JSON(http.StatusOK, res)
}

func (c *ClientController) GetUserByEmail(ctx *gin.Context) {
	Email := ctx.Param("email")
	client, err := c.clientService.FindEmail(Email)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	ctx.JSON(http.StatusOK, client)
}
