package repository

import (
	"context"
	"errors"

	"github.com/Nina-99/yottu-api/models"
)

// ErrAuthTokenNotFound signals a hash matching no token row.
var ErrAuthTokenNotFound = errors.New("auth token not found")

// AuthTokenRepository persists single-use email-flow tokens. Only token hashes
// are ever read or written.
type AuthTokenRepository interface {
	Create(ctx context.Context, token *models.AuthToken) error

	// FindByHash returns the token row for a digest, or (nil, nil) when absent.
	FindByHash(ctx context.Context, hash string) (*models.AuthToken, error)

	// Consume marks the token used ONLY if it is still unused, returning whether
	// this call won the race. The CAS guarantees single-use under concurrency.
	Consume(ctx context.Context, id uint) (bool, error)

	// InvalidateOutstanding marks every still-unused token for
	// (userType, userID, purpose) as used, so issuing a fresh token retires the
	// previous one.
	InvalidateOutstanding(ctx context.Context, userType string, userID uint, purpose string) error
}
