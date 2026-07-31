package service

import "github.com/Nina-99/yottu-api/models"

type NotificationService interface {
	Create(clientID uint, notifType, title, message, data string) error
	GetByClientID(clientID uint) ([]models.Notification, error)
	GetUnreadByClientID(clientID uint) ([]models.Notification, error)
	MarkAsRead(notificationID uint) error
	MarkAllAsRead(clientID uint) error
	CountUnread(clientID uint) (int64, error)
	RegisterPushDevice(clientID uint, pushToken, platform string) error
	UnregisterPushDevice(token string) error
	SendPushToClient(clientID uint, title, body, dataJSON string) error
}
