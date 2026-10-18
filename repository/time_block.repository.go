package repository

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
)

// TimeBlockRepository persists unavailability blocks (breaks, holidays).
//
// Listing is CENTRE-scoped and DATE-ranged on purpose: availability always asks
// for one centre (or a batch) over a bounded window, never "all blocks".
type TimeBlockRepository interface {
	Create(block *models.TimeBlock) error
	FindByID(id uint) (*models.TimeBlock, error)
	Delete(id uint) error
	// ListByCentersRange returns every block of the given centres whose Date
	// falls within [from, to] (dates compared by day).
	ListByCentersRange(centerIDs []uint, from, to time.Time) ([]models.TimeBlock, error)
}
