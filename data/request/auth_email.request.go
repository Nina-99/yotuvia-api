package request

// VerifyEmailRequest is the body of POST /auth-accounts/verify-email: the raw
// token from the verification link.
type VerifyEmailRequest struct {
	Token string `json:"token" validate:"required"`
}

// ResendVerificationRequest is the body of POST
// /auth-accounts/resend-verification. The response never reveals whether the
// email exists.
type ResendVerificationRequest struct {
	Email string `json:"email" validate:"required,email,max=70"`
}

// ForgotPasswordRequest is the body of POST /auth-accounts/forgot-password. Like
// resend, the response never reveals whether the email exists.
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email,max=70"`
}

// ResetPasswordRequest is the body of POST /auth-accounts/reset-password. The
// password uses the SAME rules as registration (min 6, max 255).
type ResetPasswordRequest struct {
	Token    string `json:"token" validate:"required"`
	Password string `json:"password" validate:"required,min=6,max=255"`
}
