package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// stubDoer returns a canned response without touching the network.
type stubDoer struct {
	status int
	body   string
	gotURL string
	// finalURL is what Canonicalize should read back as the post-redirect URL.
	// Empty means "no redirect": the input URL is echoed.
	finalURL string
	err      error
}

func (s *stubDoer) Do(req *http.Request) (*http.Response, error) {
	s.gotURL = req.URL.String()
	if s.err != nil {
		return nil, s.err
	}
	final := req.URL
	if s.finalURL != "" {
		if parsed, err := url.Parse(s.finalURL); err == nil {
			final = parsed
		}
	}
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Request:    &http.Request{URL: final},
	}, nil
}

func TestOEmbedResolver_ParsesTikTok(t *testing.T) {
	doer := &stubDoer{
		status: http.StatusOK,
		body:   `{"title":"Look","thumbnail_url":"https://cdn/t.jpg","author_name":"studio","author_url":"https://www.tiktok.com/@studio"}`,
	}
	resolver := NewHTTPOEmbedResolver(doer)

	meta, err := resolver.Resolve(context.Background(), "https://www.tiktok.com/@u/video/1", ProviderTikTok)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if meta.Title != "Look" || meta.ThumbnailURL != "https://cdn/t.jpg" || meta.AuthorName != "studio" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	if !strings.Contains(doer.gotURL, "tiktok.com/oembed?url=") {
		t.Fatalf("wrong oembed endpoint: %s", doer.gotURL)
	}
}

func TestOEmbedResolver_NonOKStatusErrors(t *testing.T) {
	resolver := NewHTTPOEmbedResolver(&stubDoer{status: http.StatusNotFound, body: "nope"})
	if _, err := resolver.Resolve(context.Background(), "https://youtu.be/abc", ProviderYouTube); err == nil {
		t.Fatal("a non-200 oembed response must error so the caller can fall back")
	}
}

func TestOEmbedResolver_UnsupportedProvider(t *testing.T) {
	resolver := NewHTTPOEmbedResolver(&stubDoer{status: http.StatusOK, body: "{}"})
	if _, err := resolver.Resolve(context.Background(), "https://www.instagram.com/reel/x/", ProviderInstagram); err != ErrOEmbedUnsupported {
		t.Fatalf("instagram must report unsupported, got %v", err)
	}
}

func TestOEmbedResolver_CanonicalizeFollowsRedirect(t *testing.T) {
	doer := &stubDoer{
		status:   http.StatusOK,
		finalURL: "https://www.tiktok.com/@studio/video/7312345678",
	}
	resolver := NewHTTPOEmbedResolver(doer)

	got := resolver.Canonicalize(context.Background(), "https://vt.tiktok.com/ZSABC123/")
	if got != "https://www.tiktok.com/@studio/video/7312345678" {
		t.Fatalf("Canonicalize: got %q", got)
	}
	if doer.gotURL != "https://vt.tiktok.com/ZSABC123/" {
		t.Fatalf("Canonicalize must request the original url, got %q", doer.gotURL)
	}
}

func TestOEmbedResolver_CanonicalizeEchoesInputWhenUnchanged(t *testing.T) {
	// A canonical link does not redirect, so the input must come back untouched.
	resolver := NewHTTPOEmbedResolver(&stubDoer{status: http.StatusOK})
	const raw = "https://www.tiktok.com/@u/video/1"
	if got := resolver.Canonicalize(context.Background(), raw); got != raw {
		t.Fatalf("Canonicalize must echo the input, got %q", got)
	}
}

func TestOEmbedResolver_CanonicalizeEchoesInputOnError(t *testing.T) {
	resolver := NewHTTPOEmbedResolver(&stubDoer{err: errors.New("network down")})
	const raw = "https://vt.tiktok.com/ZSABC123/"
	if got := resolver.Canonicalize(context.Background(), raw); got != raw {
		t.Fatalf("Canonicalize must fall back to the input on error, got %q", got)
	}
}
