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
	ClientID  uint   `gorm:"not null;index"`
	PushToken string `gorm:"size:255;not null;uniqueIndex"`
	Platform  string `gorm:"size:20"` // ios, android, web

	// Web Push only: the subscription's `keys.p256dh` / `keys.auth`.
	P256dh *string `gorm:"size:255"`
	Auth   *string `gorm:"size:255"`

	Client Client `gorm:"foreignKey:ClientID"`
}
