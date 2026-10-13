package utils

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

// Google's credential-validation endpoints. Kept as package variables (instead
// of string literals at the call site) so tests can point them at an httptest
// server without any network access.
var (
	// GoogleUserInfoURL validates OAuth 2.0 access tokens.
	GoogleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
	// GoogleTokenInfoURL validates OpenID Connect ID tokens (JWTs).
	GoogleTokenInfoURL = "https://oauth2.googleapis.com/tokeninfo"
)

// VerifyGoogleToken validates a Google credential and returns the identity it
// proves. Two credential kinds reach this function — the two clients produce
// them naturally — and they are verified at different Google endpoints:
//
//   - an OAuth 2.0 access token (opaque, e.g. the web GSI/token-client flow)
//     is checked at the userinfo endpoint; and
//   - an OpenID Connect ID token (a JWT, e.g. the native Google Sign-In flow)
//     is checked at the tokeninfo endpoint, which validates the signature,
//     expiry and audience server-side.
//
// The kind is inferred from the token shape and an unverified token is never
// trusted. Both failure paths return the same error so the HTTP contract
// (401 with this message) does not change.
func VerifyGoogleToken(token string) (*GoogleUserInfo, error) {
	if looksLikeJWT(token) {
		return verifyGoogleIDToken(token)
	}
	return verifyGoogleAccessToken(token)
}

// verifyGoogleAccessToken validates an opaque OAuth 2.0 access token via the
// userinfo endpoint (unchanged legacy behaviour).
func verifyGoogleAccessToken(accessToken string) (*GoogleUserInfo, error) {
	resp, err := GlobalHTTPClient.Get(fmt.Sprintf("%s?access_token=%s", GoogleUserInfoURL, url.QueryEscape(accessToken)))
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

// googleIDTokenClaims is the subset of Google's tokeninfo response the service
// consumes, mapped onto the same GoogleUserInfo shape used by the access-token
// path (notably `sub` -> ID and `email_verified` -> VerifiedEmail).
type googleIDTokenClaims struct {
	Subject       string       `json:"sub"`
	Email         string       `json:"email"`
	EmailVerified flexibleBool `json:"email_verified"`
	Name          string       `json:"name"`
	GivenName     string       `json:"given_name"`
	FamilyName    string       `json:"family_name"`
	Picture       string       `json:"picture"`
}

// verifyGoogleIDToken validates an OpenID Connect ID token at Google's
// tokeninfo endpoint. A non-200 response means Google rejected the token
// (bad signature, expired, wrong audience) and yields the same error as a
// rejected access token.
func verifyGoogleIDToken(idToken string) (*GoogleUserInfo, error) {
	resp, err := GlobalHTTPClient.Get(fmt.Sprintf("%s?id_token=%s", GoogleTokenInfoURL, url.QueryEscape(idToken)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("invalid google access token")
	}

	var claims googleIDTokenClaims
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return nil, err
	}

	return &GoogleUserInfo{
		ID:            claims.Subject,
		Email:         claims.Email,
		VerifiedEmail: bool(claims.EmailVerified),
		Name:          claims.Name,
		GivenName:     claims.GivenName,
		FamilyName:    claims.FamilyName,
		Picture:       claims.Picture,
	}, nil
}

// flexibleBool decodes a JSON boolean that Google may emit either as a real
// boolean or as the string "true"/"false" (tokeninfo has historically returned
// both shapes). Anything else is treated as false rather than failing the
// whole verification.
type flexibleBool bool

func (b *flexibleBool) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(strings.TrimSpace(string(data)), `"`)
	switch strings.ToLower(raw) {
	case "true", "1":
		*b = true
	default:
		*b = false
	}
	return nil
}

// looksLikeJWT reports whether token is an OpenID Connect ID token: three
// non-empty dot-separated segments whose header decodes to JSON carrying an
// "alg" member. The header check keeps an opaque access token that happens to
// contain two dots from being misrouted to the tokeninfo endpoint.
func looksLikeJWT(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return false
	}

	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(header, &fields); err != nil {
		return false
	}
	_, ok := fields["alg"]
	return ok
}

// VerifyFacebookToken validates the Access Token by colling Facebook's Graph, API
func VerifyFacebookToken(accessToken string) (*FacebookUserInfo, error) {
	graphURL := fmt.Sprintf("http://graph.facebook.com/me?fields=id,email,first_name,last_name,picture.type(large)&access_token=%s", accessToken)
	resp, err := GlobalHTTPClient.Get(graphURL)
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
