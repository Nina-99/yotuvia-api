package request

type RegisterRequest struct {
	FirstName string  `json:"first_name" validate:"omitempty,max=50"`
	LastName  string  `json:"last_name" validate:"omitempty,max=50"`
	Email     string  `json:"email" validate:"required,email,max=70"`
	Phone     string  `json:"phone" validate:"required,max=20"`
	Password  string  `json:"password" validate:"required,min=8,max=255"`
	Username  string  `json:"username" validate:"omitempty,max=50"`
	UserType  string  `json:"user_type" validate:"required,oneof=client personnel beauty_center"`
	Address   string  `json:"address" validate:"omitempty,max=255"`
	Latitude  float64 `json:"latitude" validate:"omitempty"`
	Longitude float64 `json:"longitude" validate:"omitempty"`
	// TermsAccepted is the client's assertion that the user was shown the
	// Terms and accepted them. `required` rejects false, so registration
	// without an explicit acceptance answers 400. The server stamps the
	// timestamp and the authoritative version; the client never sends those.
	TermsAccepted bool `json:"terms_accepted" validate:"required"`
}

type LoginRequest struct {
	Identifier string `json:"identifier" validate:"required"`
	Password   string `json:"password" validate:"required"`
	UserType   string `json:"user_type" validate:"omitempty,oneof=client personnel beauty_center"`
}

type SocialLoginRequest struct {
	Provider    string `json:"provider" validate:"required,oneof=google facebook"`
	AccessToken string `json:"access_token" validate:"required"`
	UserType    string `json:"user_type" validate:"required,oneof=client personnel beauty_center"`
}

type RegisterPersonnelRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	FirstName      string `json:"first_name" validate:"required,max=50"`
	LastName       string `json:"last_name" validate:"required,max=50"`
	Email          string `json:"email" validate:"required,email,max=70"`
	Phone          string `json:"phone" validate:"required,max=20"`
	Password       string `json:"password" validate:"required,min=8,max=255"`
	IsActive       *bool  `json:"is_active"`
}

// RefreshRequest is the body of POST /auth-accounts/refresh: the opaque
// refresh token to rotate.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LogoutRequest is the body of POST /auth-accounts/logout (D3): the refresh
// token of the session to revoke.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}
