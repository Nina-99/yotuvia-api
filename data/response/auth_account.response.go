package response

type AuthAccountResponse struct {
	ID         uint            `json:"id"`
	ClientID   uint            `json:"client_id"`
	Provider   string          `json:"provider"`
	ProviderID string          `json:"provider_id"`
	Client     *ClientResponse `json:"client,omitempty"`
}
