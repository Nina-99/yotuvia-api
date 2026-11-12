package request

// RequestLoginCodeRequest is the body of POST /auth-accounts/otp/request.
type RequestLoginCodeRequest struct {
	Email string `json:"email" validate:"required,email,max=70"`
}

// VerifyLoginCodeRequest is the body of POST /auth-accounts/otp/verify.
type VerifyLoginCodeRequest struct {
	Email string `json:"email" validate:"required,email,max=70"`
	Code  string `json:"code" validate:"required,len=6,numeric"`
}

// CompleteRegistrationRequest is the body of
// POST /auth-accounts/register/complete. The email is NOT part of it: it is
// taken from the registration ticket, which is the only proof that the caller
// controls the address.
type CompleteRegistrationRequest struct {
	RegistrationTicket string `json:"registration_ticket" validate:"required"`
	UserType           string `json:"user_type" validate:"required,oneof=client beauty_center"`
	FirstName          string `json:"first_name" validate:"omitempty,max=50"`
	LastName           string `json:"last_name" validate:"omitempty,max=50"`
	Phone              string `json:"phone" validate:"omitempty,max=20"`
	// Name is the beauty centre's display name.
	Name      string  `json:"name" validate:"omitempty,max=100"`
	Address   string  `json:"address" validate:"omitempty,max=255"`
	Latitude  float64 `json:"latitude" validate:"omitempty"`
	Longitude float64 `json:"longitude" validate:"omitempty"`
	// TermsAccepted must be true; the server stamps timestamp and version.
	TermsAccepted bool `json:"terms_accepted" validate:"required"`
}
