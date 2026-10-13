package repository

import "github.com/Nina-99/yotuvia-api/models"

// PlatformAdminRepository is the storage port for the platform-level principal.
// It is deliberately tiny: the account is created by the idempotent seed and
// resolved by login; there is no public registration, update or delete surface.
type PlatformAdminRepository interface {
	Create(admin *models.PlatformAdmin) error
	// FindByEmail returns the admin for an exact email match, or (nil, nil) when
	// no such admin exists.
	FindByEmail(email string) (*models.PlatformAdmin, error)
}
