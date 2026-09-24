package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
	"time"
)

// notificationListLimit caps the LIST queries (FindByClientID,
// FindByPersonnelID, FindByCenterID) so a long-lived account cannot return an
// unbounded result set to the mobile screen. The cap keeps the newest rows
// (created_at DESC) and leaves the unread COUNT untouched: the badge is served
// by a separate COUNT(*) query, so it still reflects the true unread total
// even though the list itself is finite. True cursor/offset pagination is a
// deliberate follow-up — this constant is the interim bound, not an oversight.
const notificationListLimit = 100

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
		Limit(notificationListLimit).
		Find(&notifications).Error
	return notifications, err
}

// FindUnreadByClientID is intentionally NOT capped (unlike the read list
// finders): its result feeds the same screen as the badge count, which comes
// from CountUnread's uncapped COUNT(*). Capping only the list would let the
// list show 100 rows while the badge reads e.g. 350 — a count/list desync that
// is worse than a large payload. Both sides stay uncapped until real pagination
// lands (see notificationListLimit).
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
		Limit(notificationListLimit).
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

// FindByCenterID returns center notifications scoped by BeautyCenterID, newest
// first. Center rows use the ClientID=0 sentinel with PersonnelID nil, so the
// beauty_center_id filter is sufficient and client/staff rows (BeautyCenterID
// null) never match (design D2).
func (r *NotificationRepositoryImpl) FindByCenterID(centerID uint) ([]models.Notification, error) {
	var notifications []models.Notification
	err := r.db.Where("beauty_center_id = ?", centerID).
		Order("created_at DESC").
		Limit(notificationListLimit).
		Find(&notifications).Error
	return notifications, err
}

func (r *NotificationRepositoryImpl) FindUnreadByCenterID(centerID uint) ([]models.Notification, error) {
	var notifications []models.Notification
	err := r.db.Where("beauty_center_id = ? AND is_read = ?", centerID, false).
		Order("created_at DESC").
		Find(&notifications).Error
	return notifications, err
}

func (r *NotificationRepositoryImpl) CountUnreadByCenterID(centerID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Notification{}).
		Where("beauty_center_id = ? AND is_read = ?", centerID, false).
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
