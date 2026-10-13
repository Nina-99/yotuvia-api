package utils

import (
	"encoding/base64"
	"testing"
)

func key32(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 1)
	}
	return k
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := key32(t)
	secret := "rft.example-token-value"
	enc, err := EncryptSecret(key, secret)
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	if enc == secret {
		t.Fatal("ciphertext equals plaintext")
	}
	got, err := DecryptSecret(key, enc)
	if err != nil {
		t.Fatalf("DecryptSecret: %v", err)
	}
	if got != secret {
		t.Fatalf("round trip = %q, want %q", got, secret)
	}
}

func TestEncryptUsesFreshNonce(t *testing.T) {
	key := key32(t)
	a, _ := EncryptSecret(key, "same")
	b, _ := EncryptSecret(key, "same")
	if a == b {
		t.Fatal("same plaintext produced identical ciphertext (nonce not random)")
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	enc, _ := EncryptSecret(key32(t), "secret")
	other := key32(t)
	other[0] ^= 0xFF
	if _, err := DecryptSecret(other, enc); err == nil {
		t.Fatal("decrypt with the wrong key must fail")
	}
}

func TestTokenEncryptionKeyParsing(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString(key32(t))
	t.Setenv("TIKTOK_TOKEN_KEY", valid)
	if _, err := TokenEncryptionKey(); err != nil {
		t.Fatalf("valid key rejected: %v", err)
	}

	t.Setenv("TIKTOK_TOKEN_KEY", "")
	if _, err := TokenEncryptionKey(); err != ErrTokenKeyMissing {
		t.Fatalf("empty key: want ErrTokenKeyMissing, got %v", err)
	}

	t.Setenv("TIKTOK_TOKEN_KEY", base64.StdEncoding.EncodeToString([]byte("short")))
	if _, err := TokenEncryptionKey(); err != ErrTokenKeyMissing {
		t.Fatalf("short key: want ErrTokenKeyMissing, got %v", err)
	}
}
