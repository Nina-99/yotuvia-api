package controller

import (
	"net/http"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
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

	result, err := c.authAccountService.Register(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, toAuthResponse(result))
}

func (c *AuthAccountController) Login(ctx *gin.Context) {
	var req request.LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := c.authAccountService.Login(req)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, toAuthResponse(result))
}

func (c *AuthAccountController) SocialLogin(ctx *gin.Context) {
	var req request.SocialLoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := c.authAccountService.SocialLogin(req)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, toAuthResponse(result))
}

func (c *AuthAccountController) RegisterPersonnel(ctx *gin.Context) {
	var req request.RegisterPersonnelRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.authAccountService.RegisterPersonnel(req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Created"})
}

func (c *AuthAccountController) CheckEmail(ctx *gin.Context) {
	var req request.CheckEmailRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	exists, err := c.authAccountService.CheckEmail(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"exists": exists})
}

func toAuthResponse(result service.AuthResult) response.AuthResponse {
	return response.AuthResponse{
		Token: result.Token,
		User: response.UserResponse{
			ID:        result.UserID,
			Email:     result.Email,
			FirstName: result.FirstName,
			LastName:  result.LastName,
			Role:      result.Role,
		},
	}
}
