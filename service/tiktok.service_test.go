package service

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
)

// ─── fakes ──────────────────────────────────────────────────────────────────

type fakeTikTokRepo struct {
	accounts map[uint]*models.TikTokAccount
	states   map[string]*models.TikTokConnectionState
}

func newFakeRepo() *fakeTikTokRepo {
	return &fakeTikTokRepo{
		accounts: map[uint]*models.TikTokAccount{},
		states:   map[string]*models.TikTokConnectionState{},
	}
}

func (r *fakeTikTokRepo) Upsert(a *models.TikTokAccount) error {
	cp := *a
	r.accounts[a.BeautyCenterID] = &cp
	return nil
}
func (r *fakeTikTokRepo) FindByActor(centerID, _ uint) (*models.TikTokAccount, error) {
	if a, ok := r.accounts[centerID]; ok {
		cp := *a
		return &cp, nil
	}
	return nil, nil
}
func (r *fakeTikTokRepo) DeleteByActor(centerID, _ uint) error {
	delete(r.accounts, centerID)
	return nil
}
func (r *fakeTikTokRepo) CreateState(s *models.TikTokConnectionState) error {
	cp := *s
	r.states[s.State] = &cp
	return nil
}
func (r *fakeTikTokRepo) ConsumeState(value string) (*models.TikTokConnectionState, error) {
	s, ok := r.states[value]
	if !ok {
		return nil, nil
	}
	delete(r.states, value)
	return s, nil
}

type fakeTikTokClient struct {
	exchangedCode string
	revoked       bool
	token         *TikTokToken
}

func (c *fakeTikTokClient) ExchangeCode(_ context.Context, _ TikTokConfig, code string) (*TikTokToken, error) {
	c.exchangedCode = code
	if c.token != nil {
		return c.token, nil
	}
	return &TikTokToken{AccessToken: "act", RefreshToken: "rft", OpenID: "oid", Scope: tiktokScopes, ExpiresIn: 86400, RefreshExpiresIn: 31536000}, nil
}
func (c *fakeTikTokClient) Refresh(_ context.Context, _ TikTokConfig, _ string) (*TikTokToken, error) {
	return c.token, nil
}
func (c *fakeTikTokClient) Revoke(_ context.Context, _ TikTokConfig, _ string) error {
	c.revoked = true
	return nil
}
func (c *fakeTikTokClient) GetUserInfo(_ context.Context, _ string) (*TikTokUserInfo, error) {
	return &TikTokUserInfo{OpenID: "oid", DisplayName: "Studio", AvatarURL: "https://a/x.jpg"}, nil
}
func (c *fakeTikTokClient) ListVideos(_ context.Context, _ string, _ int, _ int64) ([]TikTokVideo, int64, bool, error) {
	return []TikTokVideo{{ID: "1", Title: "t", ShareURL: "https://www.tiktok.com/@s/video/1"}}, 0, false, nil
}

// fakeFeedSvc embeds the interface so only CreateFromExternal needs defining.
type fakeFeedSvc struct {
	FeedPostService
	got request.CreateExternalFeedPostRequest
}

func (f *fakeFeedSvc) CreateFromExternal(_ FeedActor, req request.CreateExternalFeedPostRequest) (response.FeedPostResponse, error) {
	f.got = req
	return response.FeedPostResponse{ID: 1, Provider: ProviderTikTok, ExternalURL: req.ExternalURL}, nil
}

func newTikTokSUT(t *testing.T) (*TikTokServiceImpl, *fakeTikTokRepo, *fakeTikTokClient, *fakeFeedSvc) {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	repo := newFakeRepo()
	client := &fakeTikTokClient{}
	feed := &fakeFeedSvc{}
	svc := NewTikTokServiceImpl(repo, client, feed, enabledCfg(), key).(*TikTokServiceImpl)
	return svc, repo, client, feed
}

var centerFeedActor = FeedActor{Role: "beauty_center", CenterID: 3}
var staffFeedActor = FeedActor{Role: "personnel", UserID: 7}

// ─── tests ──────────────────────────────────────────────────────────────────

func TestTikTokService_DisabledWithoutKey(t *testing.T) {
	svc := NewTikTokServiceImpl(newFakeRepo(), &fakeTikTokClient{}, &fakeFeedSvc{}, enabledCfg(), nil)
	if svc.Enabled() {
		t.Fatal("service must be disabled when the encryption key is missing")
	}
	if _, err := svc.ConnectURL(centerFeedActor); err != ErrTikTokDisabled {
		t.Fatalf("want ErrTikTokDisabled, got %v", err)
	}
	acct, err := svc.Account(centerFeedActor)
	if err != nil || acct.Enabled || acct.Connected {
		t.Fatalf("disabled account = %+v, err=%v", acct, err)
	}
}

func TestTikTokService_ConnectURLCreatesState(t *testing.T) {
	svc, repo, _, _ := newTikTokSUT(t)

	raw, err := svc.ConnectURL(centerFeedActor)
	if err != nil {
		t.Fatalf("ConnectURL: %v", err)
	}
	parsed, _ := url.Parse(raw)
	if parsed.Query().Get("state") == "" {
		t.Fatalf("authorize url missing state: %s", raw)
	}
	if len(repo.states) != 1 {
		t.Fatalf("expected one stored state, got %d", len(repo.states))
	}
}

func TestTikTokService_ConnectStaffRejected(t *testing.T) {
	svc, _, _, _ := newTikTokSUT(t)
	if _, err := svc.ConnectURL(staffFeedActor); err == nil {
		t.Fatal("staff connect must be rejected in this slice")
	}
}

func TestTikTokService_CallbackStoresEncryptedTokens(t *testing.T) {
	svc, repo, client, _ := newTikTokSUT(t)

	if _, err := svc.ConnectURL(centerFeedActor); err != nil {
		t.Fatalf("ConnectURL: %v", err)
	}
	var state string
	for s := range repo.states {
		state = s
	}

	path, err := svc.HandleCallback(context.Background(), "the-code", state, "")
	if err != nil {
		t.Fatalf("HandleCallback: %v", err)
	}
	if !strings.Contains(path, "tiktok=connected") {
		t.Fatalf("unexpected redirect path: %s", path)
	}
	if client.exchangedCode != "the-code" {
		t.Fatalf("code not exchanged: %q", client.exchangedCode)
	}

	account := repo.accounts[3]
	if account == nil {
		t.Fatal("account not stored")
	}
	if account.AccessToken == "act" || account.RefreshToken == "rft" {
		t.Fatal("tokens stored in plaintext")
	}
	dec, err := utils.DecryptSecret(svc.key, account.AccessToken)
	if err != nil || dec != "act" {
		t.Fatalf("access token did not round-trip: %q err=%v", dec, err)
	}
	// The state is single-use.
	if _, err := svc.HandleCallback(context.Background(), "the-code", state, ""); err == nil {
		t.Fatal("replayed state must be rejected")
	}
}

func TestTikTokService_CallbackRejectsExpiredState(t *testing.T) {
	svc, repo, _, _ := newTikTokSUT(t)
	if err := repo.CreateState(&models.TikTokConnectionState{
		State:     "stale",
		ExpiresAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("CreateState: %v", err)
	}
	if _, err := svc.HandleCallback(context.Background(), "code", "stale", ""); err == nil {
		t.Fatal("expired state must be rejected")
	}
}

func TestTikTokService_ImportValidatesHost(t *testing.T) {
	svc, _, _, feed := newTikTokSUT(t)
	// Connect first so an account exists.
	if _, err := svc.ConnectURL(centerFeedActor); err != nil {
		t.Fatalf("ConnectURL: %v", err)
	}
	var state string
	for s := range svc.AccountRepository.(*fakeTikTokRepo).states {
		state = s
	}
	if _, err := svc.HandleCallback(context.Background(), "c", state, ""); err != nil {
		t.Fatalf("HandleCallback: %v", err)
	}

	if _, err := svc.Import(context.Background(), centerFeedActor, request.TikTokImportRequest{
		VideoID:  "1",
		ShareURL: "https://example.com/watch?v=1",
		Title:    "x",
	}); err == nil {
		t.Fatal("non-tiktok share url must be rejected")
	}

	if _, err := svc.Import(context.Background(), centerFeedActor, request.TikTokImportRequest{
		VideoID:  "1",
		ShareURL: "https://www.tiktok.com/@s/video/1",
		Title:    "Look",
	}); err != nil {
		t.Fatalf("Import: %v", err)
	}
	if feed.got.ExternalURL != "https://www.tiktok.com/@s/video/1" || feed.got.Caption != "Look" {
		t.Fatalf("unexpected import payload: %+v", feed.got)
	}
}

func TestTikTokService_DisconnectRevokesAndDeletes(t *testing.T) {
	svc, repo, client, _ := newTikTokSUT(t)
	if _, err := svc.ConnectURL(centerFeedActor); err != nil {
		t.Fatalf("ConnectURL: %v", err)
	}
	var state string
	for s := range repo.states {
		state = s
	}
	if _, err := svc.HandleCallback(context.Background(), "c", state, ""); err != nil {
		t.Fatalf("HandleCallback: %v", err)
	}

	if err := svc.Disconnect(context.Background(), centerFeedActor); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if !client.revoked {
		t.Fatal("revoke was not called")
	}
	if repo.accounts[3] != nil {
		t.Fatal("account not deleted")
	}
}
