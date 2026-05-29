package utils

import (
	"errors"
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
	Role     string `json:"role"`
	Email    string `json:"email,omitempty"`
	Username string `json:"username,omitempty"`

	jwt.RegisteredClaims
}

func GenerateJWT(userID uint, role string, email string, username string) (string, error) {
	claims := &JWTClaim{
		UserID:   userID,
		Role:     role,
		Email:    email,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
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
	return claims, nil
}
