package controller

import (
	"net/http"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
)

type AuthAccountController struct {
	authAccountService service.AuthAccountService
}

func NewAuthAccountController(authAccountService service.AuthAccountService) *AuthAccountController {
	return &AuthAccountController{authAccountService: authAccountService}
}

func (c *AuthAccountController) Register(ctx *gin.Context) {
	var req request.RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, err := c.authAccountService.Register(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{"token": token})
}

func (c *AuthAccountController) Login(ctx *gin.Context) {
	var req request.LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := c.authAccountService.Login(req)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"token": token})
}

func (c *AuthAccountController) SocialLogin(ctx *gin.Context) {
	var req request.SocialLoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := c.authAccountService.SocialLogin(req)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"token": token})
}
