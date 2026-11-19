package controller

import (
	"net/http"
	"strconv"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
)

type FavoriteController struct {
	FavoriteService service.FavoriteService
}

func NewFavoriteController(service service.FavoriteService) *FavoriteController {
	return &FavoriteController{FavoriteService: service}
}

// writeError maps service errors through apperrors.MapToHTTP so ownership
// mismatches become 403 {"error":"forbidden"}, missing rows 404 {"error":"not
// found"} and unexpected failures a generic 500 — internal error text never
// reaches the client.
func (c *FavoriteController) writeError(ctx *gin.Context, err error) {
	status, msg := apperrors.MapToHTTP(err)
	ctx.JSON(status, gin.H{"error": msg})
}

func (c *FavoriteController) Create(ctx *gin.Context) {
	var req request.CreateFavoriteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeBadRequest(ctx, err)
		return
	}

	// Retrieve ClientID from context (set by JWT middleware)
	clientID, exists := ctx.Get("clientID")
	if exists && clientID != nil {
		req.ClientID = clientID.(uint)
	}

	if err := c.FavoriteService.Create(req); err != nil {
		writeInternalError(ctx, err)
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

	// The owner is the caller's token claim: the service rejects any favorite
	// whose ClientID is not this client's id.
	if err := c.FavoriteService.Delete(uint(id), ctx.GetUint("clientID")); err != nil {
		c.writeError(ctx, err)
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

	// The service only serves a client's OWN list (clientID == token claim).
	favorites, err := c.FavoriteService.FindByClientID(uint(clientID), ctx.GetUint("clientID"))
	if err != nil {
		c.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, favorites)
}
