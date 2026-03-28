package utils

import (
	"errors"
	"os"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/golang-jwt/jwt/v5"
)

// var jwtKey = []byte(os.Getenv("JWT_SECRET"))
func getJWTSecret() ([]byte, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, errors.New("JWT_SECRET not configure")
	}
	return []byte(secret), nil
}

type JWTClaim struct {
	ClientID uint
	Email    string
	Username string

	jwt.RegisteredClaims
}

func GenerateJWT(client *models.Client) (string, error) {
	claims := &JWTClaim{
		ClientID: client.ID,
		Email:    client.Email,
		Username: client.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
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
