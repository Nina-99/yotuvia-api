package middleware

import (
	"context"
	"os"
	"strconv"
	"time"
)

// SessionActivityChecker answers the one question the auth middleware needs:
// may the session behind this access token still authenticate a request?
//
// It is an interface (not a *gorm.DB) so the middleware stays free of the
// repository layer and stays testable with a fake.
type SessionActivityChecker interface {
	IsActive(ctx context.Context, sessionID uint, now time.Time, grace time.Duration) (bool, error)
}

// sessionChecker is the process-wide checker wired once at boot. A nil checker
// means "no session verification" — the state test routers and any embedding
// that does not use SetupRouter run in. Production always wires it in
// router.SetupRouter.
var sessionChecker SessionActivityChecker

// SetSessionChecker installs the checker used by JWTAuthMiddleware. Passing nil
// disables session verification (tests only).
func SetSessionChecker(c SessionActivityChecker) {
	sessionChecker = c
}

// sessionRevokeGrace resolves how long a rotated-but-revoked generation keeps
// accepting its access token, so an in-flight request is not 401'd while the
// client swaps in the token returned by /refresh. It does NOT delay real
// revocations: those leave the session family without a live generation and are
// rejected immediately.
//
// Defaults to 30s; SESSION_REVOKE_GRACE_SEC overrides it. A malformed or
// negative value falls back to the default rather than disabling the check.
func sessionRevokeGrace() time.Duration {
	const defaultGrace = 30 * time.Second
	raw := os.Getenv("SESSION_REVOKE_GRACE_SEC")
	if raw == "" {
		return defaultGrace
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return defaultGrace
	}
	return time.Duration(n) * time.Second
}

// SessionCheckerWired reports whether a checker is installed. Exposed so a
// wiring test can prove SetupRouter does not leave the gate silently disabled: a
// nil checker is indistinguishable from "every session is valid", which is the
// failure mode this control exists to prevent.
func SessionCheckerWired() bool { return sessionChecker != nil }
