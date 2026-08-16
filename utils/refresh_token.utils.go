package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

// AccessTokenTTL is the JWT access-token validity (spec: 15 minutes).
const AccessTokenTTL = 15 * time.Minute

// RefreshTokenTTL is the refresh-token / session validity (spec: 30 days).
const RefreshTokenTTL = 30 * 24 * time.Hour

// randomBytes returns n cryptographically secure random bytes.
func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// GenerateRefreshToken returns a URL-safe opaque token derived from 32 random
// bytes. The raw token is returned to the client exactly once and is never
// persisted — only its SHA-256 digest is stored (D1).
func GenerateRefreshToken() (string, error) {
	b, err := randomBytes(32)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateFamilyID returns a random session-family identifier (16 bytes hex)
// grouping rotation generations of one device session (D1).
func GenerateFamilyID() (string, error) {
	b, err := randomBytes(16)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashRefreshToken returns the SHA-256 hex digest of a refresh token. Only the
// digest is ever stored or compared, so a leaked database never exposes a
// usable token.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
