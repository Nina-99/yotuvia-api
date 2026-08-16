package controller

import (
	"net/http"

	"github.com/Nina-99/yottu-api/apperrors"
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

// Refresh rotates the presented refresh token (public — the access token is
// expired by design when this endpoint is needed). Reuse revokes the family
// and answers 401; every other invalid token also answers 401.
func (c *AuthAccountController) Refresh(ctx *gin.Context) {
	var req request.RefreshRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	result, err := c.authAccountService.Refresh(ctx.Request.Context(), req.RefreshToken)
	if err != nil {
		status, msg := apperrors.MapToHTTP(err)
		ctx.JSON(status, gin.H{"error": msg})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"token": result.AccessToken, "refresh_token": result.RefreshToken})
}

// Logout revokes the session presenting the body's refresh token. Idempotent:
// repeated or unknown tokens still answer 204 for the authenticated caller.
func (c *AuthAccountController) Logout(ctx *gin.Context) {
	var req request.LogoutRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := c.authAccountService.Logout(ctx.Request.Context(), req.RefreshToken); err != nil {
		status, msg := apperrors.MapToHTTP(err)
		ctx.JSON(status, gin.H{"error": msg})
		return
	}

	ctx.JSON(http.StatusNoContent, nil)
}

func toAuthResponse(result service.AuthResult) response.AuthResponse {
	return response.AuthResponse{
		Token:        result.Token,
		RefreshToken: result.RefreshToken,
		User: response.UserResponse{
			ID:        result.UserID,
			Email:     result.Email,
			FirstName: result.FirstName,
			LastName:  result.LastName,
			Role:      result.Role,
		},
	}
}
