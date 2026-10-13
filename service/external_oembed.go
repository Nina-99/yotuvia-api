package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// ExternalVideoMetadata is the subset of an external video's oEmbed response
// that Yotuvia stores and shows. External videos are LINKED, never rehosted.
type ExternalVideoMetadata struct {
	Provider     string
	Title        string
	ThumbnailURL string
	AuthorName   string
	AuthorURL    string
}

// HTTPDoer is the minimal HTTP surface the resolver needs, so tests can inject
// a stub instead of hitting the network.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// OEmbedResolver fetches public metadata for an external video URL.
type OEmbedResolver interface {
	Resolve(ctx context.Context, rawURL, provider string) (*ExternalVideoMetadata, error)
}

// ErrOEmbedUnsupported means the provider has no public, keyless oEmbed (e.g.
// Instagram), so a link cannot be enriched and must be rejected or stored bare.
var ErrOEmbedUnsupported = errors.New("oembed: provider has no public oembed endpoint")

// HTTPOEmbedResolver is the production resolver.
type HTTPOEmbedResolver struct {
	client HTTPDoer
}

func NewHTTPOEmbedResolver(client HTTPDoer) *HTTPOEmbedResolver {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPOEmbedResolver{client: client}
}

// oembedEndpoint builds the keyless oEmbed URL for a provider, or "" when the
// provider has none (Instagram requires an access token).
func oembedEndpoint(provider, rawURL string) string {
	q := url.QueryEscape(rawURL)
	switch provider {
	case ProviderTikTok:
		return "https://www.tiktok.com/oembed?url=" + q
	case ProviderYouTube:
		return "https://www.youtube.com/oembed?url=" + q + "&format=json"
	default:
		return ""
	}
}

func (r *HTTPOEmbedResolver) Resolve(ctx context.Context, rawURL, provider string) (*ExternalVideoMetadata, error) {
	endpoint := oembedEndpoint(provider, rawURL)
	if endpoint == "" {
		return nil, ErrOEmbedUnsupported
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oembed: unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		Title        string `json:"title"`
		ThumbnailURL string `json:"thumbnail_url"`
		AuthorName   string `json:"author_name"`
		AuthorURL    string `json:"author_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("oembed: decode: %w", err)
	}

	return &ExternalVideoMetadata{
		Provider:     provider,
		Title:        payload.Title,
		ThumbnailURL: payload.ThumbnailURL,
		AuthorName:   payload.AuthorName,
		AuthorURL:    payload.AuthorURL,
	}, nil
}
