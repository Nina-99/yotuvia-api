package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
	"time"
)

type NotificationRepositoryImpl struct {
	db *gorm.DB
}

func NewNotificationRepositoryImpl(db *gorm.DB) NotificationRepository {
	return &NotificationRepositoryImpl{db: db}
}

func (r *NotificationRepositoryImpl) Create(notification *models.Notification) error {
	return r.db.Create(notification).Error
}

func (r *NotificationRepositoryImpl) FindByClientID(clientID uint) ([]models.Notification, error) {
	var notifications []models.Notification
	err := r.db.Where("client_id = ?", clientID).
		Order("created_at DESC").
		Find(&notifications).Error
	return notifications, err
}

func (r *NotificationRepositoryImpl) FindUnreadByClientID(clientID uint) ([]models.Notification, error) {
	var notifications []models.Notification
	err := r.db.Where("client_id = ? AND is_read = ?", clientID, false).
		Order("created_at DESC").
		Find(&notifications).Error
	return notifications, err
}

// FindByID implements NotificationRepository — used by the service to verify
// notification ownership before MarkAsRead.
// FindByPersonnelID returns staff notifications scoped by PersonnelID, newest
// first. Staff rows use the ClientID=0 sentinel, so the personnel_id filter is
// sufficient and client rows (PersonnelID null) never match.
func (r *NotificationRepositoryImpl) FindByPersonnelID(personnelID uint) ([]models.Notification, error) {
	var notifications []models.Notification
	err := r.db.Where("personnel_id = ?", personnelID).
		Order("created_at DESC").
		Find(&notifications).Error
	return notifications, err
}

func (r *NotificationRepositoryImpl) FindUnreadByPersonnelID(personnelID uint) ([]models.Notification, error) {
	var notifications []models.Notification
	err := r.db.Where("personnel_id = ? AND is_read = ?", personnelID, false).
		Order("created_at DESC").
		Find(&notifications).Error
	return notifications, err
}

func (r *NotificationRepositoryImpl) CountUnreadByPersonnelID(personnelID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Notification{}).
		Where("personnel_id = ? AND is_read = ?", personnelID, false).
		Count(&count).Error
	return count, err
}

func (r *NotificationRepositoryImpl) FindByID(notificationID uint) (*models.Notification, error) {
	var notif models.Notification
	if err := r.db.First(&notif, notificationID).Error; err != nil {
		return nil, err
	}
	return &notif, nil
}

func (r *NotificationRepositoryImpl) MarkAsRead(notificationID uint) error {
	return r.db.Model(&models.Notification{}).
		Where("id = ?", notificationID).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": time.Now(),
		}).Error
}

func (r *NotificationRepositoryImpl) MarkAllAsRead(clientID uint) error {
	return r.db.Model(&models.Notification{}).
		Where("client_id = ? AND is_read = ?", clientID, false).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": time.Now(),
		}).Error
}

func (r *NotificationRepositoryImpl) CountUnread(clientID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Notification{}).
		Where("client_id = ? AND is_read = ?", clientID, false).
		Count(&count).Error
	return count, err
}
