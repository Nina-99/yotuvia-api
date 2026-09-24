package repository

import (
	"time"

	"github.com/Nina-99/yottu-api/models"
)

// WaitlistSlot identifies one concrete queue slot (center + date + start).
// It is a query helper for the batch position lookup, not a persisted model.
type WaitlistSlot struct {
	BeautyCenterID uint
	Date           time.Time
	StartTime      string
}

type WaitlistRepository interface {
	Create(entry *models.WaitlistEntry) error
	Update(entry *models.WaitlistEntry) error
	FindByID(entryID uint) (*models.WaitlistEntry, error)
	// OfferNextWaiting atomically claims the first WAITING row for a slot
	// (FIFO by created_at): it selects with a row lock and marks it OFFERED
	// inside one transaction, so two concurrent cancellations can never offer
	// the same slot twice. Returns nil when the queue is empty.
	OfferNextWaiting(centerID uint, date time.Time, startTime string, offeredAt, expiresAt time.Time) (*models.WaitlistEntry, error)
	// FindOfferedExpired returns OFFERED entries past their claim window.
	FindOfferedExpired(now time.Time) ([]models.WaitlistEntry, error)
	// FindLive returns a client's WAITING/OFFERED entries, newest first.
	FindLive(clientID uint) ([]models.WaitlistEntry, error)
	// FindAllByClient returns every entry the client ever had (any status),
	// newest first.
	FindAllByClient(clientID uint) ([]models.WaitlistEntry, error)
	// FindLiveForSlots returns every LIVE (WAITING/OFFERED) entry for the
	// given slots, ordered by created_at ASC, so the queue rank of a page of
	// entries can be derived in one query instead of a COUNT per row.
	FindLiveForSlots(slots []WaitlistSlot) ([]models.WaitlistEntry, error)
	// ExistsLive reports a duplicate WAITING/OFFERED join for the slot.
	ExistsLive(centerID, clientID uint, date time.Time, startTime string) (bool, error)
}
