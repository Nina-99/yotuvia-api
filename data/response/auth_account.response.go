package response

type UserResponse struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
}

type AuthResponse struct {
	Token        string       `json:"token"`
	RefreshToken string       `json:"refresh_token"`
	User         UserResponse `json:"user"`
	// SessionsEvicted reports how many older sessions were revoked because the
	// account+role hit its concurrent-session cap (MAX_ACTIVE_SESSIONS, default
	// 2). Always present; 0 is the normal case, so clients that ignore it are
	// unaffected.
	SessionsEvicted int `json:"sessions_evicted"`
}

// Existing DTOs
type AuthAccountResponse struct {
	ID         uint            `json:"id"`
	ClientID   uint            `json:"client_id"`
	Provider   string          `json:"provider"`
	ProviderID string          `json:"provider_id"`
	Client     *ClientResponse `json:"client,omitempty"`
}
