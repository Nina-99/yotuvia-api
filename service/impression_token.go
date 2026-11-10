package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	// ErrInvalidImpressionToken means the token is malformed or its signature
	// does not match: the price or campaign was tampered with.
	ErrInvalidImpressionToken = errors.New("invalid impression token")
	// ErrExpiredImpressionToken means the billing window of the served slot is
	// over; the impression is never charged.
	ErrExpiredImpressionToken = errors.New("expired impression token")
)

// ImpressionPayload is what the server signs when a promoted slot is served and
// what it verifies when the client reports the view. The price is carried inside
// the signature so a client can never choose its own cost.
type ImpressionPayload struct {
	CampaignID    uint   `json:"campaign_id"`
	PriceCPMMilli int64  `json:"price_cpm_milli"`
	Nonce         string `json:"nonce"`
	Exp           int64  `json:"exp"`
}

// SignImpressionToken returns base64(payload).base64(HMAC-SHA256(payload)).
func SignImpressionToken(secret []byte, payload ImpressionPayload) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + signature, nil
}

// VerifyImpressionToken checks the signature and the expiry, returning the
// payload only when both pass.
func VerifyImpressionToken(secret []byte, token string) (ImpressionPayload, error) {
	var payload ImpressionPayload
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return payload, ErrInvalidImpressionToken
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(parts[0]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return payload, ErrInvalidImpressionToken
	}

	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return payload, ErrInvalidImpressionToken
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return payload, ErrInvalidImpressionToken
	}
	if payload.CampaignID == 0 || payload.Nonce == "" {
		return payload, ErrInvalidImpressionToken
	}
	if time.Now().Unix() > payload.Exp {
		return payload, ErrExpiredImpressionToken
	}
	return payload, nil
}

// newImpressionNonce returns a 128-bit random nonce for one served slot.
func newImpressionNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
