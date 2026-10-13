package repository

import "github.com/Nina-99/yotuvia-api/models"

type TokenTopUpRepository interface {
	Create(topup *models.TokenTopUp) error
	Update(topup *models.TokenTopUp) error
	FindByID(id uint) (*models.TokenTopUp, error)
	FindByBeautyCenterID(beautyCenterID uint) ([]models.TokenTopUp, error)
	FindByCenterIDWithStatus(beautyCenterID uint, page, limit int, status string) ([]models.TokenTopUp, int64, error)
	FindPending() ([]models.TokenTopUp, error)
	FindAll(page, limit int) ([]models.TokenTopUp, int64, error)
	// FindAllWithFilters is the platform-wide queue read: it lists top-ups across
	// EVERY centre, with an optional centre filter (nil = all) and an optional
	// status filter ("" = all). Both filters are applied in the SQL query, so the
	// caller never has to re-filter in memory and `total` always matches.
	FindAllWithFilters(page, limit int, beautyCenterID *uint, status string) ([]models.TokenTopUp, int64, error)
}
