package utils

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// var jwtKey = []byte(os.Getenv("JWT_SECRET"))
func getJWTSecret() ([]byte, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, errors.New("JWT_SECRET not configured")
	}
	return []byte(secret), nil
}

type JWTClaim struct {
	UserID   uint   `json:"user_id,omitempty"`
	CenterID uint   `json:"center_id,omitempty"`
	Role     string `json:"role"`
	Email    string `json:"email,omitempty"`
	Username string `json:"username,omitempty"`

	// SessionID ties the access token to the session row that issued it, so a
	// revocation (logout, logout-all, password reset, deactivated account,
	// session-cap eviction) takes effect on the NEXT request instead of waiting
	// out the 15-minute token lifetime. Validated by the auth middleware against
	// UserSessionRepository.IsActive.
	SessionID string `json:"sid,omitempty"`

	// Purpose is set only on non-session tokens (e.g. the registration ticket).
	// Access tokens never carry it. ValidateToken rejects any token that has it
	// so a registration ticket can never be replayed as a bearer credential.
	Purpose string `json:"purpose,omitempty"`

	jwt.RegisteredClaims
}

// GenerateJWT signs an access token bound to sessionID. The session id is
// mandatory: a token without one cannot be revoked and would be rejected by the
// auth middleware.
func GenerateJWT(userID uint, centerID uint, role string, email string, username string, sessionID string) (string, error) {
	claims := &JWTClaim{
		UserID:    userID,
		CenterID:  centerID,
		Role:      role,
		Email:     email,
		Username:  username,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessTokenTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secret, err := getJWTSecret()
	if err != nil {
		return "", err
	}
	return token.SignedString(secret)
}

func ValidateToken(signedToken string) (*JWTClaim, error) {
	token, err := jwt.ParseWithClaims(
		signedToken,
		&JWTClaim{},
		func(token *jwt.Token) (interface{}, error) {
			// Algorithm confusion guard: only HS256 is accepted. Pointer identity
			// rejects alg=none, RS/ES/PS (type mismatch), and HS384/HS512 (different
			// method pointer) before the secret is ever used.
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Method.Alg())
			}
			secret, err := getJWTSecret()
			if err != nil {
				return nil, err
			}
			return secret, nil
		},
	)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*JWTClaim)
	if !ok || !token.Valid {
		return nil, err
	}
	// A registration ticket (or any non-session token) must never authenticate
	// a request. Access tokens always carry a role and never a purpose.
	if claims.Purpose != "" || claims.Role == "" {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
