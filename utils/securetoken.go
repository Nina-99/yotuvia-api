package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"strings"
)

// ErrTokenKeyMissing is returned when TIKTOK_TOKEN_KEY is unset or malformed.
// The integration refuses to persist tokens without it, so a refresh token
// (valid 365 days on behalf of a real TikTok account) is NEVER stored in
// plaintext by accident.
var ErrTokenKeyMissing = errors.New("utils: TIKTOK_TOKEN_KEY must be a base64-encoded 32-byte key")

// TokenEncryptionKey reads and validates the AES-256 key from TIKTOK_TOKEN_KEY.
func TokenEncryptionKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("TIKTOK_TOKEN_KEY"))
	if raw == "" {
		return nil, ErrTokenKeyMissing
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, ErrTokenKeyMissing
	}
	return key, nil
}

// EncryptSecret seals plaintext with AES-256-GCM and returns base64(nonce||ct).
// A fresh random nonce is used per call, so encrypting the same token twice
// yields different ciphertexts.
func EncryptSecret(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptSecret reverses EncryptSecret.
func DecryptSecret(key []byte, encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("utils: ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
