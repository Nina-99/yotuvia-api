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

	// VerifyEmail redeems an email-verification token: single-use and
	// time-bounded. Invalid/expired/used tokens all return ErrInvalidToken.
	VerifyEmail(ctx context.Context, rawToken string) error
	// ResendVerification issues a fresh verification token when an unverified
	// account exists and sends the email. It is deliberately silent about
	// whether the account exists (returns nil either way).
	ResendVerification(ctx context.Context, req request.ResendVerificationRequest) error
	// ForgotPassword sends a reset link when an account exists, and is silent
	// about whether it does (returns nil either way).
	ForgotPassword(ctx context.Context, req request.ForgotPasswordRequest) error
	// ResetPassword redeems a single-use reset token, sets the new password and
	// revokes every session of that account (all devices, all families).
	ResetPassword(ctx context.Context, req request.ResetPasswordRequest) error
}
