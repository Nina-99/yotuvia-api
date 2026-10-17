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

// OEmbedResolver fetches public metadata for an external video URL and
// canonicalizes provider short links.
type OEmbedResolver interface {
	Resolve(ctx context.Context, rawURL, provider string) (*ExternalVideoMetadata, error)
	// Canonicalize follows a provider share/short link (e.g. TikTok's
	// vt.tiktok.com / vm.tiktok.com / tiktok.com/t/... forms) to the canonical
	// post URL, returning rawURL unchanged when it is already canonical or
	// cannot be resolved. Callers MUST bound ctx.
	Canonicalize(ctx context.Context, rawURL string) string
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

// Canonicalize follows a provider share/short link to its canonical post URL so
// a post id can be extracted from a link the user copied from the mobile app.
// Best-effort: it returns rawURL unchanged when the request fails or the
// provider does not redirect. Redirects are followed by the underlying client;
// the final URL is read from the response's Request.
func (r *HTTPOEmbedResolver) Canonicalize(ctx context.Context, rawURL string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return rawURL
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return rawURL
	}
	defer resp.Body.Close()
	if resp.Request != nil && resp.Request.URL != nil {
		if final := resp.Request.URL.String(); final != "" {
			return final
		}
	}
	return rawURL
}
