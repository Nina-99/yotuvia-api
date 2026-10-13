package utils

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The two clients naturally produce different tokens: the web app hands the
// backend an OAuth 2.0 access token (userinfo endpoint), while native mobile
// can only always guarantee an OpenID Connect ID token (a JWT, verified at
// Google's tokeninfo endpoint). VerifyGoogleToken must route each kind to the
// right endpoint and never send a token to the wrong one.

// preserveGoogleEndpoints restores the production Google endpoints once the
// test that repointed them at local httptest servers finishes.
func preserveGoogleEndpoints(t *testing.T) {
	t.Helper()
	userInfoURL := GoogleUserInfoURL
	tokenInfoURL := GoogleTokenInfoURL
	t.Cleanup(func() {
		GoogleUserInfoURL = userInfoURL
		GoogleTokenInfoURL = tokenInfoURL
	})
}

// fakeGoogleIDToken builds a structurally valid (but cryptographically fake)
// JWT so looksLikeJWT routes it to the ID-token path.
func fakeGoogleIDToken(t *testing.T) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"123","email":"a@b.c","aud":"client"}`))
	return header + "." + payload + ".fake-signature"
}

// TestVerifyGoogleTokenAccessTokenUsesUserInfoEndpoint is the existing
// contract: an opaque access token is validated at the userinfo endpoint.
func TestVerifyGoogleTokenAccessTokenUsesUserInfoEndpoint(t *testing.T) {
	preserveGoogleEndpoints(t)

	var tokenInfoHits int
	tokenInfo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		tokenInfoHits++
		http.Error(w, `{"error":"unexpected"}`, http.StatusBadRequest)
	}))
	defer tokenInfo.Close()

	userInfo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("access_token"); got != "ya29.access-token" {
			t.Errorf("userinfo access_token = %q, want %q", got, "ya29.access-token")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":             "google-id-1",
			"email":          "user@example.com",
			"verified_email": true,
			"name":           "User One",
			"given_name":     "User",
			"family_name":    "One",
			"picture":        "https://img.example.com/u.png",
		})
	}))
	defer userInfo.Close()

	GoogleUserInfoURL = userInfo.URL
	GoogleTokenInfoURL = tokenInfo.URL

	info, err := VerifyGoogleToken("ya29.access-token")
	if err != nil {
		t.Fatalf("VerifyGoogleToken(access token) error = %v", err)
	}
	if info.ID != "google-id-1" || info.Email != "user@example.com" {
		t.Fatalf("unexpected user info: %+v", info)
	}
	if !info.VerifiedEmail {
		t.Errorf("VerifiedEmail = false, want true")
	}
	if info.GivenName != "User" || info.FamilyName != "One" {
		t.Errorf("name mapping wrong: %+v", info)
	}
	if tokenInfoHits != 0 {
		t.Errorf("access token must NOT hit the tokeninfo endpoint, got %d hit(s)", tokenInfoHits)
	}
}

// TestVerifyGoogleTokenIDTokenUsesTokenInfoEndpoint covers the native mobile
// credential: a JWT ID token is validated at Google's tokeninfo endpoint and
// mapped to the same GoogleUserInfo shape.
func TestVerifyGoogleTokenIDTokenUsesTokenInfoEndpoint(t *testing.T) {
	preserveGoogleEndpoints(t)

	token := fakeGoogleIDToken(t)

	var userInfoHits int
	userInfo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		userInfoHits++
		http.Error(w, `{"error":"unexpected"}`, http.StatusBadRequest)
	}))
	defer userInfo.Close()

	tokenInfo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("id_token"); got != token {
			t.Errorf("tokeninfo id_token = %q, want %q", got, token)
		}
		// tokeninfo emits email_verified as a JSON string in some responses;
		// the parser must tolerate both shapes.
		_, _ = w.Write([]byte(`{
			"sub": "google-id-9",
			"email": "id@example.com",
			"email_verified": "true",
			"name": "Id Nine",
			"given_name": "Id",
			"family_name": "Nine",
			"picture": "https://img.example.com/id.png"
		}`))
	}))
	defer tokenInfo.Close()

	GoogleUserInfoURL = userInfo.URL
	GoogleTokenInfoURL = tokenInfo.URL

	info, err := VerifyGoogleToken(token)
	if err != nil {
		t.Fatalf("VerifyGoogleToken(id token) error = %v", err)
	}
	if info.ID != "google-id-9" {
		t.Errorf("ID = %q, want sub claim %q", info.ID, "google-id-9")
	}
	if info.Email != "id@example.com" {
		t.Errorf("Email = %q, want %q", info.Email, "id@example.com")
	}
	if !info.VerifiedEmail {
		t.Errorf("VerifiedEmail = false, want true (from string \"true\")")
	}
	if info.GivenName != "Id" || info.FamilyName != "Nine" || info.Picture != "https://img.example.com/id.png" {
		t.Errorf("unexpected mapping: %+v", info)
	}
	if userInfoHits != 0 {
		t.Errorf("id token must NOT hit the userinfo endpoint, got %d hit(s)", userInfoHits)
	}
}

// TestVerifyGoogleTokenIDTokenBooleanEmailVerified accepts the boolean form
// tokeninfo sometimes returns instead of the string form.
func TestVerifyGoogleTokenIDTokenBooleanEmailVerified(t *testing.T) {
	preserveGoogleEndpoints(t)

	tokenInfo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"sub":"s","email":"e@x.com","email_verified":true}`))
	}))
	defer tokenInfo.Close()

	GoogleUserInfoURL = tokenInfo.URL
	GoogleTokenInfoURL = tokenInfo.URL

	info, err := VerifyGoogleToken(fakeGoogleIDToken(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.VerifiedEmail {
		t.Errorf("VerifiedEmail = false, want true (boolean form)")
	}
}

// TestVerifyGoogleTokenRejectsInvalidTokens pins the failure semantics: a
// rejected access token, a rejected ID token and an unstructured garbage value
// all fail with a non-nil error and no partial result.
func TestVerifyGoogleTokenRejectsInvalidTokens(t *testing.T) {
	preserveGoogleEndpoints(t)

	rejecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"invalid_token"}`, http.StatusBadRequest)
	}))
	defer rejecting.Close()

	GoogleUserInfoURL = rejecting.URL
	GoogleTokenInfoURL = rejecting.URL

	cases := []struct {
		name  string
		token string
	}{
		{"invalid access token", "ya29.invalid"},
		{"invalid id token", fakeGoogleIDToken(t)},
		{"garbage", "definitely-not-a-token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := VerifyGoogleToken(tc.token)
			if err == nil {
				t.Fatalf("expected an error for %q, got user info %+v", tc.name, info)
			}
			if info != nil {
				t.Errorf("expected nil user info on failure, got %+v", info)
			}
		})
	}
}

// TestVerifyGoogleTokenOpaqueTokenWithDotsStaysOnUserInfo guards the routing
// heuristic: a non-empty string with two dots that is NOT a decodable JWT
// header must still be treated as an opaque access token.
func TestVerifyGoogleTokenOpaqueTokenWithDotsStaysOnUserInfo(t *testing.T) {
	preserveGoogleEndpoints(t)

	var tokenInfoHits int
	tokenInfo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		tokenInfoHits++
		http.Error(w, `{"error":"unexpected"}`, http.StatusBadRequest)
	}))
	defer tokenInfo.Close()

	userInfo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"opaque","email":"opaque@example.com"}`))
	}))
	defer userInfo.Close()

	GoogleUserInfoURL = userInfo.URL
	GoogleTokenInfoURL = tokenInfo.URL

	info, err := VerifyGoogleToken("opaque.token.value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ID != "opaque" {
		t.Errorf("ID = %q, want %q", info.ID, "opaque")
	}
	if tokenInfoHits != 0 {
		t.Errorf("non-JWT token must NOT hit tokeninfo, got %d hit(s)", tokenInfoHits)
	}
}
