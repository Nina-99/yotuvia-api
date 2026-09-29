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
		status, msg := apperrors.MapToHTTP(err)
		ctx.JSON(status, gin.H{"error": msg})
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
		status, msg := apperrors.MapToHTTP(err)
		ctx.JSON(status, gin.H{"error": msg})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Created"})
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

// VerifyEmail redeems a verification token: 200 on success, 400 for an
// unknown/expired/used token (uniform message).
func (c *AuthAccountController) VerifyEmail(ctx *gin.Context) {
	var req request.VerifyEmailRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := c.authAccountService.VerifyEmail(ctx.Request.Context(), req.Token); err != nil {
		status, msg := apperrors.MapToHTTP(err)
		ctx.JSON(status, gin.H{"error": msg})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "email verified"})
}

// ResendVerification ALWAYS answers 200 for a well-formed email, whether or not
// the account exists or is already verified — no enumeration signal.
func (c *AuthAccountController) ResendVerification(ctx *gin.Context) {
	var req request.ResendVerificationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := c.authAccountService.ResendVerification(ctx.Request.Context(), req); err != nil {
		status, msg := apperrors.MapToHTTP(err)
		ctx.JSON(status, gin.H{"error": msg})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "if the email exists and is unverified, a verification link has been sent"})
}

// ForgotPassword ALWAYS answers 200 for a well-formed email, whether or not the
// account exists — no enumeration signal.
func (c *AuthAccountController) ForgotPassword(ctx *gin.Context) {
	var req request.ForgotPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := c.authAccountService.ForgotPassword(ctx.Request.Context(), req); err != nil {
		status, msg := apperrors.MapToHTTP(err)
		ctx.JSON(status, gin.H{"error": msg})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "if the email exists, a reset link has been sent"})
}

// ResetPassword redeems a single-use reset token, sets the new password and
// revokes every session: 200 on success, 400 for a bad token or password.
func (c *AuthAccountController) ResetPassword(ctx *gin.Context) {
	var req request.ResetPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := c.authAccountService.ResetPassword(ctx.Request.Context(), req); err != nil {
		status, msg := apperrors.MapToHTTP(err)
		ctx.JSON(status, gin.H{"error": msg})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "password reset"})
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
