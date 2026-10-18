package repository

import "github.com/Nina-99/yotuvia-api/models"

type PushDeviceRepository interface {
	Register(device *models.PushDevice) error
	// FindByOwner returns every device of ONE (role, owner): the role filter is
	// what keeps a staff device from receiving a client's push when the numeric
	// ids happen to collide.
	FindByOwner(role string, ownerID uint) ([]models.PushDevice, error)
	Unregister(token string) error
}
