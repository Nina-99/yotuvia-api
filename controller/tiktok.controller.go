package controller

import (
	"net/http"
	"os"
	"strings"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
)

// TikTokController exposes the "connect your TikTok account" flow. The connect,
// account, videos, import and disconnect endpoints are authenticated; the OAuth
// callback is public because TikTok redirects the user's browser to it.
type TikTokController struct {
	tiktokService service.TikTokService
}

func NewTikTokController(s service.TikTokService) *TikTokController {
	return &TikTokController{tiktokService: s}
}

func (c *TikTokController) disabled(ctx *gin.Context) bool {
	if !c.tiktokService.Enabled() {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "tiktok integration is not configured"})
		return true
	}
	return false
}

// Connect handles GET /integrations/tiktok/connect: returns the authorization
// URL for the SPA to navigate to.
func (c *TikTokController) Connect(ctx *gin.Context) {
	if c.disabled(ctx) {
		return
	}
	url, err := c.tiktokService.ConnectURL(feedActor(ctx))
	if err != nil {
		writeBadRequest(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"url": url})
}

// Callback handles GET /integrations/tiktok/callback. TikTok redirects the
// browser here with code+state; on success (or failure) we bounce the browser
// back to the web app with a status flag.
func (c *TikTokController) Callback(ctx *gin.Context) {
	front := strings.TrimRight(os.Getenv("FRONT_HOST"), "/")

	path, err := c.tiktokService.HandleCallback(
		ctx.Request.Context(),
		ctx.Query("code"),
		ctx.Query("state"),
		ctx.Query("error"),
	)
	if err != nil {
		ctx.Redirect(http.StatusFound, front+"/app/content?tiktok=error")
		return
	}
	ctx.Redirect(http.StatusFound, front+path)
}

// Account handles GET /integrations/tiktok/account.
func (c *TikTokController) Account(ctx *gin.Context) {
	res, err := c.tiktokService.Account(feedActor(ctx))
	if err != nil {
		writeBadRequest(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, res)
}

// Videos handles GET /integrations/tiktok/videos.
func (c *TikTokController) Videos(ctx *gin.Context) {
	if c.disabled(ctx) {
		return
	}
	res, err := c.tiktokService.Videos(ctx.Request.Context(), feedActor(ctx))
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"videos": res})
}

// Import handles POST /integrations/tiktok/import.
func (c *TikTokController) Import(ctx *gin.Context) {
	if c.disabled(ctx) {
		return
	}
	var req request.TikTokImportRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeBadRequest(ctx, err)
		return
	}
	res, err := c.tiktokService.Import(ctx.Request.Context(), feedActor(ctx), req)
	if err != nil {
		writeBadRequest(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, res)
}

// Disconnect handles DELETE /integrations/tiktok/account.
func (c *TikTokController) Disconnect(ctx *gin.Context) {
	if c.disabled(ctx) {
		return
	}
	if err := c.tiktokService.Disconnect(ctx.Request.Context(), feedActor(ctx)); err != nil {
		writeBadRequest(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "disconnected"})
}
