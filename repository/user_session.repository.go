package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
)

// Session sentinel errors. The service maps all of them to 401 at the HTTP
// boundary; each carries a distinct reason so the failure is logged, not leaked.
var (
	// ErrSessionNotFound signals a refresh token matching no session row.
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionExpired signals an active session past its 30-day expiry.
	ErrSessionExpired = errors.New("session expired")
	// ErrSessionReuse signals a token that was already rotated (theft replay or
	// a lost race): the whole family is revoked before this is returned.
	ErrSessionReuse = errors.New("session reuse detected")
)

type UserSessionRepository interface {
	Create(ctx context.Context, session *models.UserSession) error
	FindByHash(ctx context.Context, hash string) (*models.UserSession, error)

	// RotateCAS atomically claims the presented session generation: it marks
	// it rotated (revoked) ONLY if it is still the active generation for that
	// hash. Returns false when a concurrent request already rotated it
	// (RowsAffected == 0) or the presented hash no longer matches.
	RotateCAS(ctx context.Context, sessionID uint, presentedHash string) (bool, error)

	// Rotate runs the full rotation transaction (D2): SELECT ... FOR UPDATE the
	// generation presenting oldHash; an already-revoked generation (reuse) or a
	// lost CAS both revoke the entire family; an expired generation fails
	// without touching the family. issue() builds the next generation row.
	Rotate(ctx context.Context, presentedHash string, issue func(active *models.UserSession) (*models.UserSession, error)) (*models.UserSession, error)

	// RevokeByHash revokes the session row matching the hash (logout). It is
	// idempotent: unknown or already-revoked tokens are a no-op.
	RevokeByHash(ctx context.Context, hash string) error

	// RevokeFamily revokes every generation row of a session family.
	RevokeFamily(ctx context.Context, familyID string) error

	// RevokeAllForUser revokes every session of one account across every family.
	// role is required because UserID is polymorphic: client #5 and center #5
	// share the numeric id, and a password reset must only kill the resetting
	// account's sessions, never another type's.
	RevokeAllForUser(ctx context.Context, userID uint, role string) error

	// EnforceActiveLimit keeps at most `limit` LIVE sessions for one
	// (userID, role) pair and returns how many were revoked to get there. A
	// session is live when it is not revoked AND not past ExpiresAt — an
	// expired-but-unrevoked row is lifecycle debris, not a live session.
	//
	// The OLDEST live sessions (by creation time) are revoked first, so the
	// newest `limit` survive. Eviction is a plain revocation, exactly like
	// RevokeAllForUser: the row stays findable and its refresh token hits the
	// reuse path (401), it is never physically deleted.
	//
	// role is part of the key for the same polymorphic-id reason as
	// RevokeAllForUser: a client login must never evict a personnel session.
	EnforceActiveLimit(ctx context.Context, userID uint, role string, limit int, now time.Time) (int, error)
}
