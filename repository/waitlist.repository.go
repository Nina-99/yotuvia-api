package repository

import (
	"time"

	"github.com/Nina-99/yottu-api/models"
)

type WaitlistRepository interface {
	Create(entry *models.WaitlistEntry) error
	Update(entry *models.WaitlistEntry) error
	FindByID(entryID uint) (*models.WaitlistEntry, error)
	// FindWaiting returns WAITING entries for a concrete slot, FIFO by
	// creation. The freed slot goes to the first row.
	FindWaiting(centerID uint, date time.Time, startTime string) ([]models.WaitlistEntry, error)
	// FindOfferedExpired returns OFFERED entries past their claim window.
	FindOfferedExpired(now time.Time) ([]models.WaitlistEntry, error)
	// FindLive returns a client's WAITING/OFFERED entries, newest first.
	FindLive(clientID uint) ([]models.WaitlistEntry, error)
	// ExistsLive reports a duplicate WAITING/OFFERED join for the slot.
	ExistsLive(centerID, clientID uint, date time.Time, startTime string) (bool, error)
}
