package response

type PushDeviceResponse struct {
	ID        uint   `json:"id"`
	ClientID  uint   `json:"client_id"`
	PushToken string `json:"push_token"`
	Platform  string `json:"platform"`
}
