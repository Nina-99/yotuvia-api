package service

import (
	"context"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
)

// TikTokService is the "connect your TikTok account" flow (Phase 3). It is
// CENTER-scoped: the connected account belongs to the beauty center, so the
// OAuth callback (a browser redirect with no JWT) can resolve the owner purely
// from the single-use state.
//
// The whole service is inert unless TikTokConfig AND the token encryption key
// are configured: Enabled reports that, and every method returns
// ErrTikTokDisabled otherwise.
type TikTokService interface {
	Enabled() bool
	// ConnectURL creates a single-use CSRF state and returns the TikTok
	// authorization URL to send the browser to.
	ConnectURL(actor FeedActor) (string, error)
	// HandleCallback consumes the state, exchanges the code and persists the
	// encrypted tokens. It returns the frontend path to redirect to.
	HandleCallback(ctx context.Context, code, state, errParam string) (string, error)
	Account(actor FeedActor) (*response.TikTokAccountResponse, error)
	Videos(ctx context.Context, actor FeedActor) ([]response.TikTokVideoResponse, error)
	Import(ctx context.Context, actor FeedActor, req request.TikTokImportRequest) (response.FeedPostResponse, error)
	Disconnect(ctx context.Context, actor FeedActor) error
}
