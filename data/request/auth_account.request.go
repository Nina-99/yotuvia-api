package request

type AuthAccountCreateRequest struct {
	ClientID   uint   `json:"client_id" validate:"required"`
	Provider   string `json:"provider" validate:"required,oneof=local google facebook"`
	ProviderID string `json:"provider_id" validate:"required,max=255"`
}

type AuthAccountUpdateRequest struct {
	ProviderID string `json:"provider_id" validate:"max=255"`
}
