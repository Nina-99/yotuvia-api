package repository

import (
	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type PushDeviceRepositoryImpl struct {
	db *gorm.DB
}

func NewPushDeviceRepositoryImpl(db *gorm.DB) PushDeviceRepository {
	return &PushDeviceRepositoryImpl{db: db}
}

func (r *PushDeviceRepositoryImpl) Register(device *models.PushDevice) error {
	// Upsert: if token exists for different owner, update; otherwise create
	var existing models.PushDevice
	result := r.db.Where("push_token = ?", device.PushToken).First(&existing)
	if result.Error == nil {
		existing.ClientID = device.ClientID
		existing.OwnerRole = device.OwnerRole
		existing.Platform = device.Platform
		// Web subscriptions rotate their keys; a re-register must refresh them.
		existing.P256dh = device.P256dh
		existing.Auth = device.Auth
		return r.db.Save(&existing).Error
	}
	if device.OwnerRole == "" {
		device.OwnerRole = "client"
	}
	return r.db.Create(device).Error
}

func (r *PushDeviceRepositoryImpl) FindByOwner(role string, ownerID uint) ([]models.PushDevice, error) {
	if role == "" {
		role = "client"
	}
	var devices []models.PushDevice
	err := r.db.Where("client_id = ? AND owner_role = ?", ownerID, role).Find(&devices).Error
	return devices, err
}

func (r *PushDeviceRepositoryImpl) Unregister(token string, role string, ownerID uint) error {
	if token == "" || role == "" || ownerID == 0 {
		// Refuse an unscoped delete: an empty filter would wipe every device.
		return apperrors.ErrBadRequest
	}
	return r.db.Where("push_token = ? AND owner_role = ? AND client_id = ?", token, role, ownerID).
		Delete(&models.PushDevice{}).Error
}
