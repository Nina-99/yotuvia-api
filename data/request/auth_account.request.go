package request

type AuthAccountCreateRequest struct {
	ClientID   uint   `json:"client_id" binding:"required"`
	Provider   string `json:"provider" binding:"required,oneof=local google facebook"`
	ProviderID string `json:"provider_id" binding:"required,max=255"`
}

type AuthAccountUpdateRequest struct {
	ProviderID string `json:"provider_id" binding:"max=255"`
}
