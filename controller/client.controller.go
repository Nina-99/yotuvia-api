package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yottu-api/apperrors"
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

// writeError maps any service error through apperrors.MapToHTTP so clients get
// the canonical status + stable message (403 for ownership, 404 for missing,
// etc.) and internal details never leak.
func (c *ClientController) writeError(ctx *gin.Context, err error) {
	status, msg := apperrors.MapToHTTP(err)
	ctx.JSON(status, gin.H{"error": msg})
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

	if err := c.clientService.Update(uint(clientId), ctx.GetUint("clientID"), req); err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Update"})
}

// UploadAvatar accepts the client's profile image as multipart field "image",
// stores it under uploads/clients/{clientID}/ and returns the relative
// avatar_url. The destination client is taken solely from the JWT claim (no
// :id surface), so a client can never upload for someone else.
func (c *ClientController) UploadAvatar(ctx *gin.Context) {
	clientID := ctx.GetUint("clientID")
	if clientID == 0 {
		c.writeError(ctx, apperrors.ErrUnauthorized)
		return
	}

	file, err := ctx.FormFile("image")
	if err != nil {
		c.writeError(ctx, apperrors.ErrValidation)
		return
	}

	avatarURL, err := c.clientService.UploadAvatar(clientID, file)
	if err != nil {
		c.writeError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"avatar_url": avatarURL})
}

func (c *ClientController) Delete(ctx *gin.Context) {
	idParam := ctx.Param("id")
	clientId, err := strconv.Atoi(idParam)
	if err != nil || clientId <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Format of ID invalid"})
		return
	}
	if err := c.clientService.Delete(uint(clientId), ctx.GetUint("clientID")); err != nil {
		c.writeError(ctx, err)
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
		c.writeError(ctx, err)
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
		c.writeError(ctx, err)
		return
	}

	var res []response.ClientResponse
	res = append(res, response.ClientResponse{ID: client.ID, FirstName: client.FirstName, LastName: client.LastName, Username: client.Username, Email: client.Email})
	ctx.JSON(http.StatusOK, res)
}

func (c *ClientController) FindMe(ctx *gin.Context) {
	clientID := ctx.GetUint("clientID")
	if clientID == 0 {
		c.writeError(ctx, apperrors.ErrUnauthorized)
		return
	}

	client, err := c.clientService.FindByID(clientID, clientID)
	if err != nil {
		c.writeError(ctx, err)
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

	client, err := c.clientService.FindByID(uint(clientID), ctx.GetUint("clientID"))
	if err != nil {
		c.writeError(ctx, err)
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
