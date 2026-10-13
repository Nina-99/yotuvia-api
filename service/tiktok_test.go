package service

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// tiktokStubDoer captures the outgoing request and returns a canned response.
type tiktokStubDoer struct {
	status    int
	body      string
	gotMethod string
	gotURL    string
	gotBody   string
	gotAuth   string
}

func (s *tiktokStubDoer) Do(req *http.Request) (*http.Response, error) {
	s.gotMethod = req.Method
	s.gotURL = req.URL.String()
	s.gotAuth = req.Header.Get("Authorization")
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		s.gotBody = string(raw)
	}
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(strings.NewReader(s.body)),
	}, nil
}

func enabledCfg() TikTokConfig {
	return TikTokConfig{ClientKey: "ck", ClientSecret: "cs", RedirectURI: "https://app.test/cb"}
}

func TestTikTok_AuthorizeURL(t *testing.T) {
	got := enabledCfg().AuthorizeURL("state123")
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("unparseable authorize url: %v", err)
	}
	if parsed.Scheme+"://"+parsed.Host+parsed.Path != TikTokAuthorizeURL {
		t.Fatalf("wrong authorize endpoint: %s", got)
	}
	q := parsed.Query()
	for k, want := range map[string]string{
		"client_key":    "ck",
		"response_type": "code",
		"scope":         tiktokScopes,
		"redirect_uri":  "https://app.test/cb",
		"state":         "state123",
	} {
		if q.Get(k) != want {
			t.Errorf("param %s = %q, want %q", k, q.Get(k), want)
		}
	}
}

func TestTikTok_DisabledWithoutCredentials(t *testing.T) {
	client := NewHTTPTikTokClient(&tiktokStubDoer{})
	if _, err := client.ExchangeCode(context.Background(), TikTokConfig{}, "code"); err != ErrTikTokDisabled {
		t.Fatalf("want ErrTikTokDisabled, got %v", err)
	}
	if _, err := client.Refresh(context.Background(), TikTokConfig{}, "rt"); err != ErrTikTokDisabled {
		t.Fatalf("want ErrTikTokDisabled, got %v", err)
	}
	if err := client.Revoke(context.Background(), TikTokConfig{}, "at"); err != ErrTikTokDisabled {
		t.Fatalf("want ErrTikTokDisabled, got %v", err)
	}
}

func TestTikTok_ExchangeCode(t *testing.T) {
	doer := &tiktokStubDoer{body: `{"access_token":"act","expires_in":86400,"open_id":"oid","refresh_expires_in":31536000,"refresh_token":"rft","scope":"user.info.basic,video.list","token_type":"Bearer"}`}
	client := NewHTTPTikTokClient(doer)

	tok, err := client.ExchangeCode(context.Background(), enabledCfg(), "the-code")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if doer.gotMethod != http.MethodPost || doer.gotURL != TikTokTokenURL {
		t.Fatalf("wrong request: %s %s", doer.gotMethod, doer.gotURL)
	}
	form, _ := url.ParseQuery(doer.gotBody)
	for k, want := range map[string]string{
		"client_key":    "ck",
		"client_secret": "cs",
		"code":          "the-code",
		"grant_type":    "authorization_code",
		"redirect_uri":  "https://app.test/cb",
	} {
		if form.Get(k) != want {
			t.Errorf("form %s = %q, want %q", k, form.Get(k), want)
		}
	}
	if tok.AccessToken != "act" || tok.RefreshToken != "rft" || tok.OpenID != "oid" || tok.ExpiresIn != 86400 {
		t.Fatalf("unexpected token: %+v", tok)
	}
}

func TestTikTok_ExchangeCode_SurfacesErrorBody(t *testing.T) {
	doer := &tiktokStubDoer{status: http.StatusBadRequest, body: `{"error":"invalid_request","error_description":"Redirect_uri is not matched"}`}
	client := NewHTTPTikTokClient(doer)

	_, err := client.ExchangeCode(context.Background(), enabledCfg(), "code")
	if err == nil || !strings.Contains(err.Error(), "invalid_request") {
		t.Fatalf("want invalid_request error, got %v", err)
	}
}

func TestTikTok_Refresh(t *testing.T) {
	doer := &tiktokStubDoer{body: `{"access_token":"act2","refresh_token":"rft2","expires_in":86400}`}
	client := NewHTTPTikTokClient(doer)

	tok, err := client.Refresh(context.Background(), enabledCfg(), "rft")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	form, _ := url.ParseQuery(doer.gotBody)
	if form.Get("grant_type") != "refresh_token" || form.Get("refresh_token") != "rft" {
		t.Fatalf("wrong refresh form: %s", doer.gotBody)
	}
	if tok.AccessToken != "act2" || tok.RefreshToken != "rft2" {
		t.Fatalf("unexpected token: %+v", tok)
	}
}

func TestTikTok_GetUserInfo(t *testing.T) {
	doer := &tiktokStubDoer{body: `{"data":{"user":{"open_id":"o","union_id":"u","avatar_url":"https://a/x.jpg","display_name":"Studio"}},"error":{"code":"ok"}}`}
	client := NewHTTPTikTokClient(doer)

	info, err := client.GetUserInfo(context.Background(), "act")
	if err != nil {
		t.Fatalf("GetUserInfo: %v", err)
	}
	if doer.gotAuth != "Bearer act" {
		t.Fatalf("missing bearer header: %q", doer.gotAuth)
	}
	if info.DisplayName != "Studio" || info.OpenID != "o" || info.AvatarURL != "https://a/x.jpg" {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestTikTok_GetUserInfo_SurfacesErrorCode(t *testing.T) {
	doer := &tiktokStubDoer{body: `{"data":{},"error":{"code":"access_token_invalid","message":"token expired"}}`}
	client := NewHTTPTikTokClient(doer)

	_, err := client.GetUserInfo(context.Background(), "act")
	if err == nil || !strings.Contains(err.Error(), "access_token_invalid") {
		t.Fatalf("want error code surfaced, got %v", err)
	}
}

func TestTikTok_ListVideos(t *testing.T) {
	doer := &tiktokStubDoer{body: `{"data":{"videos":[{"id":"1","title":"t","video_description":"d","duration":2,"cover_image_url":"https://c/x.jpg","embed_link":"https://e/1","share_url":"https://s/1"}],"cursor":1646883959000,"has_more":true},"error":{"code":"ok"}}`}
	client := NewHTTPTikTokClient(doer)

	videos, cursor, hasMore, err := client.ListVideos(context.Background(), "act", 20, 0)
	if err != nil {
		t.Fatalf("ListVideos: %v", err)
	}
	if doer.gotMethod != http.MethodPost || doer.gotAuth != "Bearer act" {
		t.Fatalf("wrong request: %s auth=%q", doer.gotMethod, doer.gotAuth)
	}
	if !strings.Contains(doer.gotBody, `"max_count":20`) {
		t.Fatalf("missing max_count in body: %s", doer.gotBody)
	}
	if len(videos) != 1 || videos[0].ID != "1" || videos[0].EmbedLink != "https://e/1" {
		t.Fatalf("unexpected videos: %+v", videos)
	}
	if cursor != 1646883959000 || !hasMore {
		t.Fatalf("unexpected paging: cursor=%d hasMore=%v", cursor, hasMore)
	}
}

func TestTikTok_ListVideos_ClampsMaxCount(t *testing.T) {
	doer := &tiktokStubDoer{body: `{"data":{"videos":[]},"error":{"code":"ok"}}`}
	client := NewHTTPTikTokClient(doer)

	if _, _, _, err := client.ListVideos(context.Background(), "act", 500, 0); err != nil {
		t.Fatalf("ListVideos: %v", err)
	}
	if !strings.Contains(doer.gotBody, `"max_count":20`) {
		t.Fatalf("max_count not clamped: %s", doer.gotBody)
	}
}
