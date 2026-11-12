package repository

import (
	"context"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
)

// OtpChallengeRepository is the storage port for single-use email login codes.
type OtpChallengeRepository interface {
	Create(ctx context.Context, challenge *models.OtpChallenge) error

	// FindActiveByEmail returns the newest unused, unexpired challenge for an
	// email, or (nil, nil) when none matches.
	FindActiveByEmail(ctx context.Context, email string, now time.Time) (*models.OtpChallenge, error)

	// Consume marks the challenge used ONLY if it is still unused, returning
	// whether this call won the race (single-use under concurrency).
	Consume(ctx context.Context, id uint) (bool, error)

	// IncrementAttempts bumps the failure counter and returns the new value.
	IncrementAttempts(ctx context.Context, id uint) (int, error)

	// InvalidateOutstanding marks every still-unused challenge for an email as
	// used, so issuing a fresh code retires the previous one.
	InvalidateOutstanding(ctx context.Context, email string, now time.Time) error

	// CountRecent counts challenges created for an email since a point in time,
	// used to throttle request floods per address.
	CountRecent(ctx context.Context, email string, since time.Time) (int64, error)
}
