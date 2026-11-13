package repository

import (
	"context"
	"errors"

	"github.com/Nina-99/yotuvia-api/models"
)

// ErrEmailAlreadyReserved signals that another account already holds the email
// in the global registry. It is the race-proof counterpart of the service-level
// pre-check.
var ErrEmailAlreadyReserved = errors.New("email already reserved")

// AccountEmailRepository is the storage port for the global email registry.
type AccountEmailRepository interface {
	// Reserve claims an email for a role. Returns ErrEmailAlreadyReserved when
	// the unique index rejects the insert (another request won the race).
	Reserve(ctx context.Context, email, userType string) (*models.AccountEmail, error)

	// BindUser records the role row id once it exists.
	BindUser(ctx context.Context, id, userID uint) error

	// Release drops a reservation whose account creation failed.
	Release(ctx context.Context, id uint) error

	// FindByEmail returns the registry row for an email, or (nil, nil).
	FindByEmail(ctx context.Context, email string) (*models.AccountEmail, error)
}
