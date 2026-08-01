package repository

import "github.com/Nina-99/yottu-api/models"

type NotificationRepository interface {
	Create(notification *models.Notification) error
	FindByClientID(clientID uint) ([]models.Notification, error)
	FindUnreadByClientID(clientID uint) ([]models.Notification, error)
	FindByID(notificationID uint) (*models.Notification, error)
	MarkAsRead(notificationID uint) error
	MarkAllAsRead(clientID uint) error
	CountUnread(clientID uint) (int64, error)
}
