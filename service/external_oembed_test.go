package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// stubDoer returns a canned response without touching the network.
type stubDoer struct {
	status int
	body   string
	gotURL string
}

func (s *stubDoer) Do(req *http.Request) (*http.Response, error) {
	s.gotURL = req.URL.String()
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(strings.NewReader(s.body)),
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
