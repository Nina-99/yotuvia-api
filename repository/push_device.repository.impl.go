package repository

import (
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
	// Upsert: if token exists for different client, update; otherwise create
	var existing models.PushDevice
	result := r.db.Where("push_token = ?", device.PushToken).First(&existing)
	if result.Error == nil {
		existing.ClientID = device.ClientID
		existing.Platform = device.Platform
		return r.db.Save(&existing).Error
	}
	return r.db.Create(device).Error
}

func (r *PushDeviceRepositoryImpl) FindByClientID(clientID uint) ([]models.PushDevice, error) {
	var devices []models.PushDevice
	err := r.db.Where("client_id = ?", clientID).Find(&devices).Error
	return devices, err
}

func (r *PushDeviceRepositoryImpl) Unregister(token string) error {
	return r.db.Where("push_token = ?", token).Delete(&models.PushDevice{}).Error
}
