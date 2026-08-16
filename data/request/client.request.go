package request

type UpdateClientRequest struct {
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name" validate:"required"`
	Username  string `json:"username" validate:"required"`
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password"` // "" = keep existing hash
	Phone     string `json:"phone" validate:"required"`
	IsActive  bool   `json:"is_active"`
	AvatarURL string `json:"avatar_url"` // "" = keep existing value
}
