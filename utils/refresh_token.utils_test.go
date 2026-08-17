package utils

import (
	"strings"
	"testing"
)

// TestGenerateRefreshToken — the generator must produce a URL-safe opaque
// token from 32 random bytes (43 base64url chars, no padding).
func TestGenerateRefreshToken(t *testing.T) {
	token, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken returned error: %v", err)
	}
	if len(token) != 43 {
		t.Errorf("token length = %d, want 43 (32 bytes base64url)", len(token))
	}
	if strings.ContainsAny(token, "+/=") {
		t.Errorf("token contains non-URL-safe characters: %q", token)
	}
}

// TestGenerateRefreshToken_Uniqueness — two draws must never collide.
func TestGenerateRefreshToken_Uniqueness(t *testing.T) {
	a, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken returned error: %v", err)
	}
	b, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken returned error: %v", err)
	}
	if a == b {
		t.Errorf("two generated tokens are identical: %q", a)
	}
}

// TestHashRefreshToken — the digest is deterministic, fixed-size, sensitive to
// input, and never equals the raw token.
func TestHashRefreshToken(t *testing.T) {
	h := HashRefreshToken("the-raw-token")
	if len(h) != 64 {
		t.Errorf("hash length = %d, want 64 (SHA-256 hex)", len(h))
	}
	if HashRefreshToken("the-raw-token") != h {
		t.Errorf("hash must be deterministic for the same input")
	}
	if HashRefreshToken("the-raw-token-2") == h {
		t.Errorf("different inputs must produce different digests")
	}
	if h == "the-raw-token" {
		t.Errorf("hash must never equal the raw token")
	}
}

// TestHashRefreshToken_OnlyDigestStored — the stored value must not contain
// recoverable token material (digest length differs from raw length).
func TestHashRefreshToken_OnlyDigestStored(t *testing.T) {
	raw := "a-very-long-refresh-token-value"
	if len(HashRefreshToken(raw)) == len(raw) {
		t.Errorf("stored digest must not be the raw token")
	}
}
