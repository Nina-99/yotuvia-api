package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// GoogleUserInfo is the structure of the data returned by Google
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

// FacebookUserInfo is the structure of the data returned by Facebook
type FacebookUserInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Picture   struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	} `json:"picture"`
}

// GlobalHTTPClient is a reusabel client with a timeout to avoid blocking server threads
var GlobalHTTPClient = &http.Client{Timeout: 10 * time.Second}

// VerifyGoogleToken validates the Access Token by calling Google's servers
func VerifyGoogleToken(accessToken string) (*GoogleUserInfo, error) {
	resp, err := GlobalHTTPClient.Get(fmt.Sprintf("https://www.googleapis.com/oauth2/v2/userinfo?access_token=%s", accessToken))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("invalid google access token")
	}

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// VerifyFacebookToken validates the Access Token by colling Facebook's Graph, API
func VerifyFacebookToken(accessToken string) (*FacebookUserInfo, error) {
	url := fmt.Sprintf("http://graph.facebook.com/me?fields=id,email,first_name,last_name,picture.type(large)&access_token=%s", accessToken)
	resp, err := GlobalHTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("invalid facebook access token")
	}

	var userInfo FacebookUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}
