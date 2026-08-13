package repository

import "github.com/Nina-99/yottu-api/models"

type NotificationRepository interface {
	Create(notification *models.Notification) error
	FindByClientID(clientID uint) ([]models.Notification, error)
	FindUnreadByClientID(clientID uint) ([]models.Notification, error)
	// FindByPersonnelID/FindUnreadByPersonnelID/CountUnreadByPersonnelID scope
	// staff notifications by the PersonnelID column only; staff rows carry the
	// ClientID=0 sentinel (design D5) so they never collide with client queries.
	FindByPersonnelID(personnelID uint) ([]models.Notification, error)
	FindUnreadByPersonnelID(personnelID uint) ([]models.Notification, error)
	CountUnreadByPersonnelID(personnelID uint) (int64, error)
	FindByID(notificationID uint) (*models.Notification, error)
	MarkAsRead(notificationID uint) error
	MarkAllAsRead(clientID uint) error
	CountUnread(clientID uint) (int64, error)
}
