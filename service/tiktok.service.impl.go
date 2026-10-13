package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
)

// stateTTL bounds how long a user has to finish the TikTok consent screen.
const stateTTL = 10 * time.Minute

// tokenRefreshSkew refreshes slightly before expiry so an in-flight API call
// never races a token that dies mid-request.
const tokenRefreshSkew = 5 * time.Minute

type TikTokServiceImpl struct {
	AccountRepository repository.TikTokAccountRepository
	Client            TikTokClient
	// FeedPostService creates the imported feed post, reusing the exact
	// external-link path (provider gate + oEmbed enrichment).
	FeedPostService FeedPostService
	Config          TikTokConfig
	key             []byte
}

func NewTikTokServiceImpl(
	repo repository.TikTokAccountRepository,
	client TikTokClient,
	feedSvc FeedPostService,
	cfg TikTokConfig,
	key []byte,
) TikTokService {
	return &TikTokServiceImpl{
		AccountRepository: repo,
		Client:            client,
		FeedPostService:   feedSvc,
		Config:            cfg,
		key:               key,
	}
}

// Enabled reports whether the integration can run at all: credentials present
// AND a valid encryption key. Anything missing keeps it inert.
func (s *TikTokServiceImpl) Enabled() bool {
	return s.Config.Enabled() && len(s.key) == 32
}

// centerActor validates that the actor is a beauty center (the only scope
// supported in this slice) and returns its id.
func centerActor(actor FeedActor) (uint, error) {
	if actor.asStaff() {
		return 0, errors.New("tiktok: connect is available to the center account only")
	}
	if actor.CenterID == 0 {
		return 0, errors.New("tiktok: missing center claim")
	}
	return actor.CenterID, nil
}

func (s *TikTokServiceImpl) ConnectURL(actor FeedActor) (string, error) {
	if !s.Enabled() {
		return "", ErrTikTokDisabled
	}
	centerID, err := centerActor(actor)
	if err != nil {
		return "", err
	}

	state, err := randomState()
	if err != nil {
		return "", err
	}
	row := models.TikTokConnectionState{
		State:          state,
		BeautyCenterID: centerID,
		PersonnelID:    0,
		ExpiresAt:      time.Now().Add(stateTTL),
	}
	if err := s.AccountRepository.CreateState(&row); err != nil {
		return "", err
	}
	return s.Config.AuthorizeURL(state), nil
}

// HandleCallback consumes the one-time state, exchanges the code and stores the
// encrypted tokens. It returns a frontend path (relative) the controller
// redirects the browser to.
func (s *TikTokServiceImpl) HandleCallback(ctx context.Context, code, state, errParam string) (string, error) {
	if !s.Enabled() {
		return "", ErrTikTokDisabled
	}
	if errParam != "" {
		return "", fmt.Errorf("tiktok: authorization denied (%s)", errParam)
	}
	if strings.TrimSpace(state) == "" {
		return "", errors.New("tiktok: missing state")
	}

	stored, err := s.AccountRepository.ConsumeState(state)
	if err != nil {
		return "", err
	}
	if stored == nil {
		return "", errors.New("tiktok: unknown or already-used state")
	}
	if time.Now().After(stored.ExpiresAt) {
		return "", errors.New("tiktok: authorization expired")
	}

	token, err := s.Client.ExchangeCode(ctx, s.Config, code)
	if err != nil {
		return "", err
	}
	info, err := s.Client.GetUserInfo(ctx, token.AccessToken)
	if err != nil {
		return "", err
	}

	encAccess, err := utils.EncryptSecret(s.key, token.AccessToken)
	if err != nil {
		return "", err
	}
	encRefresh, err := utils.EncryptSecret(s.key, token.RefreshToken)
	if err != nil {
		return "", err
	}

	now := time.Now()
	account := &models.TikTokAccount{
		BeautyCenterID:   stored.BeautyCenterID,
		PersonnelID:      stored.PersonnelID,
		OpenID:           token.OpenID,
		DisplayName:      info.DisplayName,
		AvatarURL:        info.AvatarURL,
		AccessToken:      encAccess,
		RefreshToken:     encRefresh,
		Scope:            token.Scope,
		AccessExpiresAt:  now.Add(time.Duration(token.ExpiresIn) * time.Second),
		RefreshExpiresAt: now.Add(time.Duration(token.RefreshExpiresIn) * time.Second),
	}
	if err := s.AccountRepository.Upsert(account); err != nil {
		return "", err
	}
	return "/app/content?tiktok=connected", nil
}

func (s *TikTokServiceImpl) Account(actor FeedActor) (*response.TikTokAccountResponse, error) {
	centerID, err := centerActor(actor)
	if err != nil {
		return nil, err
	}
	if !s.Enabled() {
		return &response.TikTokAccountResponse{Enabled: false, Connected: false}, nil
	}
	account, err := s.AccountRepository.FindByActor(centerID, 0)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return &response.TikTokAccountResponse{Enabled: true, Connected: false}, nil
	}
	return &response.TikTokAccountResponse{
		Enabled:     true,
		Connected:   true,
		DisplayName: account.DisplayName,
		AvatarURL:   account.AvatarURL,
	}, nil
}

func (s *TikTokServiceImpl) Videos(ctx context.Context, actor FeedActor) ([]response.TikTokVideoResponse, error) {
	if !s.Enabled() {
		return nil, ErrTikTokDisabled
	}
	account, err := s.requireAccount(actor)
	if err != nil {
		return nil, err
	}
	access, _, err := s.freshAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}

	videos, _, _, err := s.Client.ListVideos(ctx, access, 20, 0)
	if err != nil {
		return nil, err
	}
	out := make([]response.TikTokVideoResponse, 0, len(videos))
	for _, v := range videos {
		out = append(out, response.TikTokVideoResponse{
			ID:            v.ID,
			Title:         v.Title,
			Description:   v.VideoDescription,
			Duration:      v.Duration,
			CoverImageURL: v.CoverImageURL,
			ShareURL:      v.ShareURL,
		})
	}
	return out, nil
}

func (s *TikTokServiceImpl) Import(ctx context.Context, actor FeedActor, req request.TikTokImportRequest) (response.FeedPostResponse, error) {
	if !s.Enabled() {
		return response.FeedPostResponse{}, ErrTikTokDisabled
	}
	if _, err := s.requireAccount(actor); err != nil {
		return response.FeedPostResponse{}, err
	}
	if ProviderFromURL(req.ShareURL) != ProviderTikTok {
		return response.FeedPostResponse{}, errors.New("tiktok: share url is not a TikTok video link")
	}
	caption := strings.TrimSpace(req.Title)
	return s.FeedPostService.CreateFromExternal(actor, request.CreateExternalFeedPostRequest{
		ExternalURL: req.ShareURL,
		Caption:     caption,
		ServiceID:   req.ServiceID,
		IsPromo:     req.IsPromo,
	})
}

func (s *TikTokServiceImpl) Disconnect(ctx context.Context, actor FeedActor) error {
	if !s.Enabled() {
		return ErrTikTokDisabled
	}
	centerID, err := centerActor(actor)
	if err != nil {
		return err
	}
	account, err := s.AccountRepository.FindByActor(centerID, 0)
	if err != nil {
		return err
	}
	if account == nil {
		return nil
	}
	// Best-effort revoke: even if TikTok is unreachable, the local tokens are
	// deleted so the connection is gone from this app's side.
	if access, _, decErr := s.decryptedTokens(account); decErr == nil {
		if revErr := s.Client.Revoke(ctx, s.Config, access); revErr != nil {
			utils.Log.Warnf("[TIKTOK] revoke failed for center %d: %v", centerID, revErr)
		}
	}
	return s.AccountRepository.DeleteByActor(centerID, 0)
}

func (s *TikTokServiceImpl) requireAccount(actor FeedActor) (*models.TikTokAccount, error) {
	centerID, err := centerActor(actor)
	if err != nil {
		return nil, err
	}
	account, err := s.AccountRepository.FindByActor(centerID, 0)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, errors.New("tiktok: no connected account")
	}
	return account, nil
}

// freshAccessToken returns a usable access token, refreshing and persisting the
// rotated tokens when the current one is expired or about to be.
func (s *TikTokServiceImpl) freshAccessToken(ctx context.Context, account *models.TikTokAccount) (string, *models.TikTokAccount, error) {
	access, refresh, err := s.decryptedTokens(account)
	if err != nil {
		return "", nil, err
	}
	if time.Now().Add(tokenRefreshSkew).Before(account.AccessExpiresAt) {
		return access, account, nil
	}

	token, err := s.Client.Refresh(ctx, s.Config, refresh)
	if err != nil {
		return "", nil, err
	}
	newAccess, err := utils.EncryptSecret(s.key, token.AccessToken)
	if err != nil {
		return "", nil, err
	}
	// TikTok may rotate the refresh token; only overwrite when a new one came back.
	newRefresh := account.RefreshToken
	if token.RefreshToken != "" {
		if newRefresh, err = utils.EncryptSecret(s.key, token.RefreshToken); err != nil {
			return "", nil, err
		}
	}
	now := time.Now()
	account.AccessToken = newAccess
	account.RefreshToken = newRefresh
	account.AccessExpiresAt = now.Add(time.Duration(token.ExpiresIn) * time.Second)
	account.RefreshExpiresAt = now.Add(time.Duration(token.RefreshExpiresIn) * time.Second)
	if err := s.AccountRepository.Upsert(account); err != nil {
		return "", nil, err
	}
	return token.AccessToken, account, nil
}

func (s *TikTokServiceImpl) decryptedTokens(account *models.TikTokAccount) (string, string, error) {
	access, err := utils.DecryptSecret(s.key, account.AccessToken)
	if err != nil {
		return "", "", fmt.Errorf("tiktok: decrypt access token: %w", err)
	}
	refresh, err := utils.DecryptSecret(s.key, account.RefreshToken)
	if err != nil {
		return "", "", fmt.Errorf("tiktok: decrypt refresh token: %w", err)
	}
	return access, refresh, nil
}

// randomState returns an unguessable URL-safe CSRF state.
func randomState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
