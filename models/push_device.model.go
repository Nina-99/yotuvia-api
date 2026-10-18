package models

import "gorm.io/gorm"

// PushDevice is one push destination for a client.
//
// Two flavours share the table because they share the SAME lifecycle (register
// on sign-in, dedupe by a unique subscription id, delete on sign-out):
//
//   - native (Platform "ios"/"android"): PushToken is the Expo push token.
//   - web (Platform "web"): PushToken is the browser PushSubscription ENDPOINT
//     (which the Web Push spec guarantees is unique per subscription), and the
//     subscription's two encryption keys live in P256dh/Auth. Without them the
//     payload cannot be encrypted, so they are required for a web device.
type PushDevice struct {
	gorm.Model
	// ClientID is the POLYMORPHIC owner id: a client id, a personnel id or a
	// centre id depending on OwnerRole. The name predates staff/owner push and
	// is kept so the existing rows and index stay valid.
	ClientID uint `gorm:"not null;index"`
	// OwnerRole scopes ClientID: "client" (default, every pre-existing row),
	// "personnel" or "beauty_center". A push lookup ALWAYS filters by both, so a
	// staff device can never receive a client's notification just because the
	// numeric ids collide.
	OwnerRole string `gorm:"size:20;not null;default:client;index"`
	PushToken string `gorm:"size:255;not null;uniqueIndex"`
	Platform  string `gorm:"size:20"` // ios, android, web

	// Web Push only: the subscription's `keys.p256dh` / `keys.auth`.
	P256dh *string `gorm:"size:255"`
	Auth   *string `gorm:"size:255"`

	Client Client `gorm:"foreignKey:ClientID"`
}
