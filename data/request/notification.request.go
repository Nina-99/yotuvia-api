package request

// RegisterPushDeviceRequest registers ONE push destination for the caller.
//
// Native: `push_token` is the Expo token and `platform` is ios|android.
// Web: `push_token` is the browser PushSubscription endpoint and `platform` is
//
//	"web", with the subscription's two encryption keys. They are OPTIONAL in the
//
// DTO (a native register never sends them) and REQUIRED at the service layer for
// a web device — a web subscription without keys cannot be encrypted.
type RegisterPushDeviceRequest struct {
	PushToken string `json:"push_token" validate:"required"`
	Platform  string `json:"platform" validate:"required"`
	// Subscription keys, Web Push only.
	P256dh string `json:"p256dh"`
	Auth   string `json:"auth"`
}
