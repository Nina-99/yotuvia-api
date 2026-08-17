package request

type RegisterRequest struct {
	FirstName string  `json:"first_name" validate:"omitempty,max=50"`
	LastName  string  `json:"last_name" validate:"omitempty,max=50"`
	Email     string  `json:"email" validate:"required,email,max=70"`
	Phone     string  `json:"phone" validate:"required,max=20"`
	Password  string  `json:"password" validate:"required,min=6,max=255"`
	Username  string  `json:"username" validate:"omitempty,max=50"`
	UserType  string  `json:"user_type" validate:"required,oneof=client personnel beauty_center"`
	Address   string  `json:"address" validate:"omitempty,max=255"`
	Latitude  float64 `json:"latitude" validate:"omitempty"`
	Longitude float64 `json:"longitude" validate:"omitempty"`
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

type CheckEmailRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type RegisterPersonnelRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	FirstName      string `json:"first_name" validate:"required,max=50"`
	LastName       string `json:"last_name" validate:"required,max=50"`
	Email          string `json:"email" validate:"required,email,max=70"`
	Phone          string `json:"phone" validate:"required,max=20"`
	Password       string `json:"password" validate:"required,min=6,max=255"`
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
