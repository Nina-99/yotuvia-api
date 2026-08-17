package repository

import (
	"time"

	"github.com/Nina-99/yottu-api/models"
)

type ReservationRepository interface {
	Create(reservation *models.Reservation) error
	Update(reservation *models.Reservation) error
	Delete(reservation *models.Reservation) error
	FindById(reservationID uint) (*models.Reservation, error)
	FindByClientID(clientID uint) ([]models.Reservation, error)
	FindUpcomingByClientID(clientID uint) (*models.Reservation, error)
	// FindByBeautyCenterId returns a center's reservations. An optional date
	// (typically a parsed YYYY-MM-DD) scopes the query to that day and orders
	// by start_time ascending; a nil date preserves the current unfiltered,
	// unordered behavior.
	FindByBeautyCenterId(beautyCenterID uint, date *time.Time) ([]models.Reservation, error)
	// FindByPersonnelId returns a staff member's reservations (agenda). from/to
	// are an optional inclusive date range; nil means open-ended on that side.
	// Results are ordered by date then start_time ascending.
	FindByPersonnelId(personnelID uint, from, to *time.Time) ([]models.Reservation, error)
	// FindAll is scoped to a single beauty center (the caller's centerID claim).
	FindAll(page, limit int, centerID uint) ([]models.Reservation, int64, error)
	CheckOverlap(centerID uint, date time.Time, startTime, endTime string, personnelID, excludeID uint) (bool, error)
	FindByToken(token string) (*models.Reservation, error)
	// CompleteVerification atomically marks a reservation COMPLETED only while
	// it is still PENDING/CONFIRMED (exactly-once under concurrent scans).
	// Returns true when this call performed the update, false when another scan
	// already completed the reservation.
	CompleteVerification(reservation *models.Reservation) (bool, error)
	UpdatePersonnelID(reservationID uint, personnelID uint) error
	DeleteReservationServices(reservationID uint) error
}
