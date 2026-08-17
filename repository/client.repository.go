package repository

import (
	"time"

	"github.com/Nina-99/yottu-api/models"
)

// ClientMetrics holds the aggregate summary for one client in the admin client
// list (design D1). Only COMPLETED reservations contribute; a client whose
// reservations at the center are all CANCELLED/PENDING keeps zeros and a nil
// LastVisit.
type ClientMetrics struct {
	VisitsCount int
	TotalSpent  float64
	LastVisit   *time.Time
}

type ClientRepository interface {
	Create(user *models.Client) error
	Update(user *models.Client) error
	Delete(user *models.Client) error
	FindById(userID uint) (*models.Client, error)
	// FindAllAdmin returns the clients that have at least one reservation (of
	// any status) at centerID — the authz scoping for GET /client (spec:
	// Client Endpoint Ownership) — with COMPLETED-only summary metrics and a
	// total scoped to that center. The old global FindAll was removed: a
	// center-scoped list must never fall back to the full client table.
	FindAllAdmin(page, limit int, centerID uint) ([]models.Client, []ClientMetrics, int64, error)
	FindByEmail(email string) (*models.Client, error)
	FindByUsername(username string) (*models.Client, error)
}
