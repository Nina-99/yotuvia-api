package utils

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Login-code lifetimes and limits.
const (
	// LoginCodeTTL is how long a 6-digit code stays valid. A login code is a
	// direct sign-in credential, so the window is intentionally short.
	LoginCodeTTL = 10 * time.Minute
	// LoginCodeMaxAttempts is the number of wrong submissions before a
	// challenge is burned.
	LoginCodeMaxAttempts = 5
	// LoginCodeRequestWindow caps how many codes one address may request in a
	// rolling window.
	LoginCodeRequestWindow = 15 * time.Minute
	// LoginCodeMaxPerWindow is the per-email request budget inside the window.
	LoginCodeMaxPerWindow = 3
	// RegistrationTicketTTL is how long a verified email can be used to finish
	// registration.
	RegistrationTicketTTL = 15 * time.Minute
)

// GenerateOTP returns a cryptographically random 6-digit code, zero-padded.
func GenerateOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// HashOTP returns the SHA-256 hex digest of a login code. Only the digest is
// persisted; the raw code travels exclusively inside the email.
func HashOTP(code string) string {
	return HashAuthToken(code)
}

// LoginCodeLink builds the deep link embedded in the login-code email. It opens
// the app on the code screen with the email and code pre-filled.
func LoginCodeLink(email, code string) string {
	return fmt.Sprintf("%slogin?email=%s&code=%s",
		DeepLinkBaseURL(),
		url.QueryEscape(email),
		url.QueryEscape(code),
	)
}

// registrationTicketClaims is the payload of a registration ticket. It carries
// ONLY the verified email: the role and profile data are supplied later, and the
// ticket itself can never mint a session.
type registrationTicketClaims struct {
	Purpose string `json:"purpose"`
	Email   string `json:"email"`
	jwt.RegisteredClaims
}

// getTicketSecret returns the registration-ticket signing key. A dedicated
// secret is recommended; JWT_SECRET is accepted as a development fallback.
func getTicketSecret() ([]byte, error) {
	if secret := os.Getenv("REGISTRATION_TICKET_SECRET"); secret != "" {
		return []byte(secret), nil
	}
	return getJWTSecret()
}

// GenerateRegistrationTicket signs a short-lived ticket for a verified email.
func GenerateRegistrationTicket(email string) (string, error) {
	claims := &registrationTicketClaims{
		Purpose: "registration",
		Email:   email,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{"register"},
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(RegistrationTicketTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secret, err := getTicketSecret()
	if err != nil {
		return "", err
	}
	return token.SignedString(secret)
}

// ParseRegistrationTicket verifies a ticket and returns the email it proves.
// Every invalid outcome collapses into one error so callers cannot probe state.
func ParseRegistrationTicket(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("invalid registration ticket")
	}
	token, err := jwt.ParseWithClaims(
		raw,
		&registrationTicketClaims{},
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Method.Alg())
			}
			return getTicketSecret()
		},
		jwt.WithAudience("register"),
	)
	if err != nil {
		return "", errors.New("invalid registration ticket")
	}
	claims, ok := token.Claims.(*registrationTicketClaims)
	if !ok || !token.Valid || claims.Purpose != "registration" || claims.Email == "" {
		return "", errors.New("invalid registration ticket")
	}
	return claims.Email, nil
}
