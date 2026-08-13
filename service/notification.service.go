package service

import "github.com/Nina-99/yottu-api/models"

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
	RegisterPushDevice(clientID uint, pushToken, platform string) error
	UnregisterPushDevice(token string) error
	SendPushToClient(clientID uint, title, body, dataJSON string) error
}
