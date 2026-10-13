package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// TikTok integration endpoints (verified against
// developers.tiktok.com/doc, 2026-08). Login Kit issues a 24h access token and
// a 365d refresh token; the Display API exposes only the AUTHENTICATED user's
// own profile and public videos. External videos are still never rehosted.
const (
	TikTokAuthorizeURL = "https://www.tiktok.com/v2/auth/authorize/"
	TikTokTokenURL     = "https://open.tiktokapis.com/v2/oauth/token/"
	TikTokRevokeURL    = "https://open.tiktokapis.com/v2/oauth/revoke/"
	TikTokUserInfoURL  = "https://open.tiktokapis.com/v2/user/info/"
	TikTokVideoListURL = "https://open.tiktokapis.com/v2/video/list/"

	// tiktokScopes match the scopes the app must be APPROVED for. Without
	// approval the app only works against its owner's own account (sandbox).
	tiktokScopes = "user.info.basic,video.list"
)

// TikTokConfig holds the OAuth app credentials. When any field is empty the
// whole integration is DISABLED and every entry point short-circuits with
// ErrTikTokDisabled, so the feature stays inert until TikTok approves the app.
type TikTokConfig struct {
	ClientKey    string
	ClientSecret string
	RedirectURI  string
}

// TikTokConfigFromEnv reads the credentials from the environment (plain
// os.Getenv, matching the rest of the codebase). Unset values disable TikTok.
func TikTokConfigFromEnv() TikTokConfig {
	return TikTokConfig{
		ClientKey:    strings.TrimSpace(os.Getenv("TIKTOK_CLIENT_KEY")),
		ClientSecret: strings.TrimSpace(os.Getenv("TIKTOK_CLIENT_SECRET")),
		RedirectURI:  strings.TrimSpace(os.Getenv("TIKTOK_REDIRECT_URI")),
	}
}

// Enabled reports whether the credentials are present.
func (c TikTokConfig) Enabled() bool {
	return c.ClientKey != "" && c.ClientSecret != "" && c.RedirectURI != ""
}

// ErrTikTokDisabled is returned when the integration has no credentials.
var ErrTikTokDisabled = fmt.Errorf("tiktok: integration is not configured")

// AuthorizeURL builds the TikTok authorization page URL for a CSRF `state`.
// disable_auto_auth=1 forces the consent screen on every connect so a user can
// always choose which account to link.
func (c TikTokConfig) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("client_key", c.ClientKey)
	q.Set("response_type", "code")
	q.Set("scope", tiktokScopes)
	q.Set("redirect_uri", c.RedirectURI)
	q.Set("state", state)
	q.Set("disable_auto_auth", "1")
	return TikTokAuthorizeURL + "?" + q.Encode()
}

// TikTokToken is a refreshable user access token.
type TikTokToken struct {
	OpenID           string
	AccessToken      string
	RefreshToken     string
	Scope            string
	ExpiresIn        int
	RefreshExpiresIn int
}

// TikTokUserInfo is the subset of the user's profile the feed shows.
type TikTokUserInfo struct {
	OpenID      string
	UnionID     string
	AvatarURL   string
	DisplayName string
}

// TikTokVideo is one item from the Display API video list.
type TikTokVideo struct {
	ID               string
	Title            string
	VideoDescription string
	Duration         int
	CoverImageURL    string
	EmbedLink        string
	ShareURL         string
}

// TikTokClient manages the OAuth token lifecycle and reads the authenticated
// user's Display API data. HTTP is injected so the whole client is unit tested
// without the network (and without real credentials).
type TikTokClient interface {
	ExchangeCode(ctx context.Context, cfg TikTokConfig, code string) (*TikTokToken, error)
	Refresh(ctx context.Context, cfg TikTokConfig, refreshToken string) (*TikTokToken, error)
	Revoke(ctx context.Context, cfg TikTokConfig, accessToken string) error
	GetUserInfo(ctx context.Context, accessToken string) (*TikTokUserInfo, error)
	ListVideos(ctx context.Context, accessToken string, maxCount int, cursor int64) ([]TikTokVideo, int64, bool, error)
}

// HTTPTikTokClient is the production TikTokClient.
type HTTPTikTokClient struct {
	client HTTPDoer
}

// NewHTTPTikTokClient builds the client; a nil HTTPDoer falls back to the
// shared http.DefaultClient.
func NewHTTPTikTokClient(client HTTPDoer) *HTTPTikTokClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPTikTokClient{client: client}
}

// tokenResponse is the token endpoint's wire shape (both grant types).
type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	OpenID           string `json:"open_id"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	RefreshToken     string `json:"refresh_token"`
	Scope            string `json:"scope"`
	TokenType        string `json:"token_type"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// ExchangeCode trades an authorization code for a user token.
func (t *HTTPTikTokClient) ExchangeCode(ctx context.Context, cfg TikTokConfig, code string) (*TikTokToken, error) {
	if !cfg.Enabled() {
		return nil, ErrTikTokDisabled
	}
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("tiktok: authorization code is required")
	}
	form := url.Values{}
	form.Set("client_key", cfg.ClientKey)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", cfg.RedirectURI)
	return t.postToken(ctx, form)
}

// Refresh renews an access token without user consent.
func (t *HTTPTikTokClient) Refresh(ctx context.Context, cfg TikTokConfig, refreshToken string) (*TikTokToken, error) {
	if !cfg.Enabled() {
		return nil, ErrTikTokDisabled
	}
	if strings.TrimSpace(refreshToken) == "" {
		return nil, fmt.Errorf("tiktok: refresh token is required")
	}
	form := url.Values{}
	form.Set("client_key", cfg.ClientKey)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	return t.postToken(ctx, form)
}

// Revoke tells TikTok to drop the app's access for a user token.
func (t *HTTPTikTokClient) Revoke(ctx context.Context, cfg TikTokConfig, accessToken string) error {
	if !cfg.Enabled() {
		return ErrTikTokDisabled
	}
	if strings.TrimSpace(accessToken) == "" {
		return fmt.Errorf("tiktok: access token is required")
	}
	form := url.Values{}
	form.Set("client_key", cfg.ClientKey)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("token", accessToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TikTokRevokeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tiktok: revoke returned status %d", resp.StatusCode)
	}
	return nil
}

// GetUserInfo reads the authenticated user's profile.
func (t *HTTPTikTokClient) GetUserInfo(ctx context.Context, accessToken string) (*TikTokUserInfo, error) {
	endpoint := TikTokUserInfoURL + "?fields=" + url.QueryEscape("open_id,union_id,avatar_url,display_name")
	resp, err := t.get(ctx, endpoint, accessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload struct {
		Data struct {
			User struct {
				OpenID      string `json:"open_id"`
				UnionID     string `json:"union_id"`
				AvatarURL   string `json:"avatar_url"`
				DisplayName string `json:"display_name"`
			} `json:"user"`
		} `json:"data"`
		Error tiktokAPIError `json:"error"`
	}
	if err := t.decode(resp, &payload, "user info"); err != nil {
		return nil, err
	}
	u := payload.Data.User
	return &TikTokUserInfo{
		OpenID:      u.OpenID,
		UnionID:     u.UnionID,
		AvatarURL:   u.AvatarURL,
		DisplayName: u.DisplayName,
	}, nil
}

// ListVideos reads the authenticated user's own public videos, newest first.
// cover_image_url values expire, so callers must not cache them long-term.
func (t *HTTPTikTokClient) ListVideos(ctx context.Context, accessToken string, maxCount int, cursor int64) ([]TikTokVideo, int64, bool, error) {
	if maxCount <= 0 || maxCount > 20 {
		maxCount = 20
	}
	endpoint := TikTokVideoListURL + "?fields=" + url.QueryEscape("id,title,video_description,duration,cover_image_url,embed_link,share_url")

	body, err := json.Marshal(struct {
		MaxCount int   `json:"max_count"`
		Cursor   int64 `json:"cursor,omitempty"`
	}{MaxCount: maxCount, Cursor: cursor})
	if err != nil {
		return nil, 0, false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, 0, false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, 0, false, err
	}
	defer resp.Body.Close()

	var payload struct {
		Data struct {
			Videos []struct {
				ID               string `json:"id"`
				Title            string `json:"title"`
				VideoDescription string `json:"video_description"`
				Duration         int    `json:"duration"`
				CoverImageURL    string `json:"cover_image_url"`
				EmbedLink        string `json:"embed_link"`
				ShareURL         string `json:"share_url"`
			} `json:"videos"`
			Cursor  int64 `json:"cursor"`
			HasMore bool  `json:"has_more"`
		} `json:"data"`
		Error tiktokAPIError `json:"error"`
	}
	if err := t.decode(resp, &payload, "video list"); err != nil {
		return nil, 0, false, err
	}

	videos := make([]TikTokVideo, 0, len(payload.Data.Videos))
	for _, v := range payload.Data.Videos {
		videos = append(videos, TikTokVideo{
			ID:               v.ID,
			Title:            v.Title,
			VideoDescription: v.VideoDescription,
			Duration:         v.Duration,
			CoverImageURL:    v.CoverImageURL,
			EmbedLink:        v.EmbedLink,
			ShareURL:         v.ShareURL,
		})
	}
	return videos, payload.Data.Cursor, payload.Data.HasMore, nil
}

// tiktokAPIError mirrors the Display API's error envelope.
type tiktokAPIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (t *HTTPTikTokClient) postToken(ctx context.Context, form url.Values) (*TikTokToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TikTokTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var payload tokenResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("tiktok: token decode (status %d): %w", resp.StatusCode, err)
	}
	if payload.Error != "" {
		return nil, fmt.Errorf("tiktok: token %s: %s", payload.Error, payload.ErrorDescription)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tiktok: token returned status %d", resp.StatusCode)
	}
	if payload.AccessToken == "" {
		return nil, fmt.Errorf("tiktok: token response missing access_token")
	}
	return &TikTokToken{
		OpenID:           payload.OpenID,
		AccessToken:      payload.AccessToken,
		RefreshToken:     payload.RefreshToken,
		Scope:            payload.Scope,
		ExpiresIn:        payload.ExpiresIn,
		RefreshExpiresIn: payload.RefreshExpiresIn,
	}, nil
}

func (t *HTTPTikTokClient) get(ctx context.Context, endpoint, accessToken string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	return t.client.Do(req)
}

// decode reads a Display API envelope, surfacing a non-ok `error.code` as a Go
// error so callers never treat an error body as data. The error envelope is
// parsed separately (generically) because `out` is a typed payload struct.
func (t *HTTPTikTokClient) decode(resp *http.Response, out any, what string) error {
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("tiktok: %s decode (status %d): %w", what, resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tiktok: %s returned status %d", what, resp.StatusCode)
	}
	var envelope struct {
		Error tiktokAPIError `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		if code := envelope.Error.Code; code != "" && code != "ok" {
			return fmt.Errorf("tiktok: %s: %s (%s)", what, envelope.Error.Message, code)
		}
	}
	return nil
}
