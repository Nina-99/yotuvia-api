package request

type RegisterRequest struct {
	FirstName string `json:"first_name" validate:"required,max=50"`
	LastName  string `json:"last_name" validate:"required,max=50"`
	Email     string `json:"email" validate:"required,email,max=70"`
	Phone     string `json:"phone" validate:"required,max=20"`
	Password  string `json:"password" validate:"required,min=6,max=255"`
	Username  string `json:"username" validate:"omitempty,max=50"` // Optional, if it doesn't come we'll use email
	UserType  string `json:"user_type" validate:"required,oneof=client personnel beauty_center"`
}

type LoginRequest struct {
	Identifier string `json:"identifier" validate:"required"`
	Password   string `json:"password" validate:"required"`
	UserType   string `json:"user_type" validate:"required,oneof=client personnel beauty_center"`
}

type SocialLoginRequest struct {
	Provider    string `json:"provider" validate:"required,oneof=google facebook"`
	AccessToken string `json:"access_token" validate:"required"`
	UserType    string `json:"user_type" validate:"required,oneof=client personnel beauty_center"`
}
