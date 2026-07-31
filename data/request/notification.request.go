package request

type RegisterPushDeviceRequest struct {
	PushToken string `json:"push_token" validate:"required"`
	Platform  string `json:"platform" validate:"required"`
}
