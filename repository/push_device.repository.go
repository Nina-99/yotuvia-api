package repository

import "github.com/Nina-99/yottu-api/models"

type PushDeviceRepository interface {
	Register(device *models.PushDevice) error
	FindByClientID(clientID uint) ([]models.PushDevice, error)
	Unregister(token string) error
}
