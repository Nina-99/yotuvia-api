package response

type UserResponse struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

// Existing DTOs
type AuthAccountResponse struct {
	ID         uint            `json:"id"`
	ClientID   uint            `json:"client_id"`
	Provider   string          `json:"provider"`
	ProviderID string          `json:"provider_id"`
	Client     *ClientResponse `json:"client,omitempty"`
}
