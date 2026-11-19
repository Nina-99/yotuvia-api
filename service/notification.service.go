package service

import "github.com/Nina-99/yotuvia-api/models"

type NotificationService interface {
	Create(clientID uint, notifType, title, message, data string) error
	// GetByClientID/GetUnreadByClientID/CountUnread/MarkAllAsRead take the path
	// clientId plus the actor's clientID claim and reject mismatches with
	// ErrForbidden; MarkAsRead verifies the notification belongs to the actor.
	GetByClientID(clientID, actorClientID uint) ([]models.Notification, error)
	GetUnreadByClientID(clientID, actorClientID uint) ([]models.Notification, error)
	MarkAsRead(notificationID, actorClientID uint) error
	MarkAllAsRead(clientID, actorClientID uint) error
	CountUnread(clientID, actorClientID uint) (int64, error)
	// Personnel-scoped notifications (design D5): rows are stored with the
	// ClientID=0 sentinel + PersonnelID set, so client queries never collide.
	// GetByPersonnelID/GetUnreadByPersonnelID/CountUnreadByPersonnelID take the
	// path personnelID plus the actor's claim and reject mismatches with
	// ErrForbidden; MarkAsReadByPersonnel verifies ownership nil-safely
	// (client rows have PersonnelID nil and can never be claimed by staff).
	CreateForPersonnel(personnelID uint, notifType, title, message, data string) error
	GetByPersonnelID(personnelID, actorPersonnelID uint) ([]models.Notification, error)
	GetUnreadByPersonnelID(personnelID, actorPersonnelID uint) ([]models.Notification, error)
	CountUnreadByPersonnelID(personnelID, actorPersonnelID uint) (int64, error)
	MarkAsReadByPersonnel(notificationID, actorPersonnelID uint) error
	// Center-scoped notifications (design D1/D3): rows are stored with the
	// ClientID=0 sentinel + BeautyCenterID set + PersonnelID nil, so client and
	// staff queries never collide. CreateForCenter is the single trigger entry
	// point for best-effort emits from other services; Get*/CountUnread take
	// the path centerID plus the actor's claim and reject mismatches with
	// ErrForbidden; MarkAsReadByCenter verifies ownership nil-safely (client
	// and staff rows have BeautyCenterID nil and can never be claimed by a
	// center).
	CreateForCenter(centerID uint, notifType, title, message, data string) error
	GetByCenterID(centerID, actorCenterID uint) ([]models.Notification, error)
	GetUnreadByCenterID(centerID, actorCenterID uint) ([]models.Notification, error)
	CountUnreadByCenterID(centerID, actorCenterID uint) (int64, error)
	MarkAsReadByCenter(notificationID, actorCenterID uint) error
	// RegisterPushDevice stores ONE device for the caller's role: "client",
	// "personnel" or "beauty_center". The role comes from the JWT, so a staff or
	// owner device is never mistaken for a client one.
	RegisterPushDevice(role string, actorID uint, pushToken, platform, p256dh, auth string) error
	UnregisterPushDevice(token string, role string, actorID uint) error
	SendPushToClient(clientID uint, title, body, dataJSON string) error
	// SendPushToOwner delivers to one (role, owner) — "personnel" or
	// "beauty_center". It is what staff and owners receive; clients keep using
	// SendPushToClient.
	SendPushToOwner(role string, ownerID uint, title, body, dataJSON string) error
	// VapidPublicKey returns the application server key a browser needs before
	// it will create a PushSubscription. Empty when Web Push is not configured.
	VapidPublicKey() string
}
