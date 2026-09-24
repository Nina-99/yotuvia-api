package service

import (
	"time"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
)

type WaitlistService interface {
	// Join takes a place in line for a concrete future slot.
	Join(req request.JoinWaitlistRequest, clientID uint) (*models.WaitlistEntry, error)
	// Leave cancels the caller's own WAITING/OFFERED entry.
	Leave(entryID, clientID uint) error
	// ListMine returns the caller's entries. With history=false it returns
	// only live (WAITING/OFFERED) entries; with history=true it returns the
	// full history, any status, newest first. Both populate Position for live
	// entries (0 for closed ones) and expire lapsed offers at read time.
	ListMine(clientID uint, history bool) ([]models.WaitlistEntry, error)
	// OfferForFreedSlot offers a freed slot to the first waiter (FIFO).
	// Returns nil when the queue is empty. Called after a cancellation and
	// by the expiry sweep for the next waiter.
	OfferForFreedSlot(centerID uint, date time.Time, startTime string) (*models.WaitlistEntry, error)
	// Claim books the offered slot for the owning client. On a taken slot
	// (conflict) the entry expires and the offer passes to the next waiter.
	Claim(entryID, clientID uint) (*models.Reservation, error)
	// SweepExpiredOffers expires lapsed offers, passes each to the next
	// waiter, and returns how many new offers were made.
	SweepExpiredOffers(now time.Time) (int, error)
}
