package service

import (
	"context"

	"github.com/Nina-99/yotuvia-api/data/request"
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
	// SessionsEvicted counts the older sessions revoked to keep this (user,
	// role) inside its concurrent-session cap. 0 is the normal case; >0 means
	// another signed-in session was ended, so the client can tell the user.
	SessionsEvicted int
}

// RefreshResult is the response of POST /auth-accounts/refresh: a fresh access
// token plus the rotated refresh token (no user payload).
type RefreshResult struct {
	AccessToken  string
	RefreshToken string
}

// LoginCodeResult is the outcome of verifying a login code. Exactly one side is
// populated: Session when the email owns an account, or RegistrationTicket when
// it does not (the caller then finishes registration).
type LoginCodeResult struct {
	Session            *AuthResult
	RegistrationTicket string
	Email              string
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
	// LogoutAll revokes EVERY session of the acting account across every
	// family and device — including the session that made the call. The caller
	// is responsible for discarding its local tokens; the endpoint still
	// answers 200.
	LogoutAll(ctx context.Context, userID uint, role string) error

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

	// RequestLoginCode sends a 6-digit sign-in code. It answers the same way
	// whether or not the email owns an account, so it is not an existence
	// oracle, and it throttles per email.
	RequestLoginCode(ctx context.Context, req request.RequestLoginCodeRequest) error
	// VerifyLoginCode consumes a challenge and either issues a session (account
	// exists and is allowed) or returns a signed registration ticket.
	VerifyLoginCode(ctx context.Context, req request.VerifyLoginCodeRequest) (LoginCodeResult, error)
	// CompleteRegistration turns a registration ticket into an account. The
	// email comes from the ticket; only client and beauty_center are allowed.
	CompleteRegistration(ctx context.Context, req request.CompleteRegistrationRequest) (AuthResult, error)
}
