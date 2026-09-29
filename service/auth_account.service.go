package service

import (
	"context"

	"github.com/Nina-99/yottu-api/data/request"
)

// AuthResult is the outcome of login/register/social. D4: it gains the raw
// refresh token issued for the new session (additive — old clients ignore it).
type AuthResult struct {
	Token        string
	RefreshToken string
	UserID       uint
	Email        string
	FirstName    string
	LastName     string
	Role         string
}

// RefreshResult is the response of POST /auth-accounts/refresh: a fresh access
// token plus the rotated refresh token (no user payload).
type RefreshResult struct {
	AccessToken  string
	RefreshToken string
}

type AuthAccountService interface {
	Register(req request.RegisterRequest) (AuthResult, error)
	Login(req request.LoginRequest) (AuthResult, error)
	SocialLogin(req request.SocialLoginRequest) (AuthResult, error)
	RegisterPersonnel(req request.RegisterPersonnelRequest) error
	// Refresh rotates the presented refresh token (D2): atomic CAS rotation;
	// reuse of an already-rotated token revokes the entire family.
	Refresh(ctx context.Context, refreshToken string) (RefreshResult, error)
	// Logout revokes the session presenting the refresh token (D3). Idempotent.
	Logout(ctx context.Context, refreshToken string) error
}
