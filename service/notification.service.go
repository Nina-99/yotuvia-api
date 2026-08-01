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
	RegisterPushDevice(clientID uint, pushToken, platform string) error
	UnregisterPushDevice(token string) error
	SendPushToClient(clientID uint, title, body, dataJSON string) error
}
