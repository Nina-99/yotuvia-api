package utils

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

// Auth-token lifetimes for the email flows.
//
//   - Email verification: 24h. A user may open the link hours later; the window
//     is generous but still bounded, and a new verification token invalidates
//     the previous one.
//   - Password reset: 1h. A reset link is a direct account-takeover credential,
//     so its window is intentionally short.
const (
	EmailVerificationTTL = 24 * time.Hour
	PasswordResetTTL     = 1 * time.Hour
)

// GenerateAuthToken returns a URL-safe opaque token derived from 32 random
// bytes (CSPRNG, 256 bits of entropy). The raw token travels only inside the
// email link and is never persisted — only its SHA-256 digest is stored.
func GenerateAuthToken() (string, error) {
	b, err := randomBytes(32)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashAuthToken returns the SHA-256 hex digest of an auth token. Lookups and
// comparisons always use the digest, so a leaked database exposes no usable
// token.
func HashAuthToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
