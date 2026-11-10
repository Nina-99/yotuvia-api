package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// stubResolver returns canned metadata without touching the network.
type stubResolver struct {
	meta        *service.ExternalVideoMetadata
	err         error
	gotProvider string
	// canonical is what Canonicalize returns; empty means "already canonical".
	canonical string
	// gotCanonical records the URL Canonicalize was asked to resolve.
	gotCanonical string
}

func (s *stubResolver) Resolve(_ context.Context, _ string, provider string) (*service.ExternalVideoMetadata, error) {
	s.gotProvider = provider
	return s.meta, s.err
}

func (s *stubResolver) Canonicalize(_ context.Context, rawURL string) string {
	s.gotCanonical = rawURL
	if s.canonical != "" {
		return s.canonical
	}
	return rawURL
}

func newExternalFeedSUT(t *testing.T, resolver service.OEmbedResolver, missions ...service.TokenMissionService) service.FeedPostService {
	t.Helper()
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedDashboardCenter(t, db, 3, "Feed Center", "feed@test.com", true)
	seedDashboardPersonnel(t, db, 7, 3, true)
	seedDashboardService(t, db, 12, 3, "Balayage", true)

	return service.NewFeedPostServiceImpl(
		repository.NewFeedPostRepositoryImpl(db),
		repository.NewBeautyCenterRepositoryImpl(db),
		repository.NewPersonnelRepositoryImpl(db),
		repository.NewServiceRepositoryImpl(db),
		repository.NewPostLikeRepositoryImpl(db),
		repository.NewSavedVideoRepositoryImpl(db),
		repository.NewVideoCommentRepositoryImpl(db),
		repository.NewPersonnelFollowRepositoryImpl(db),
		nil, // notification service not exercised here
		validator.New(),
		resolver,
		missions...,
	)
}

// TestFeed_CreateExternal_EnrichesFromOEmbed covers the happy path: the provider
// is derived from the URL and the oEmbed metadata is stored on the link.
func TestFeed_CreateExternal_EnrichesFromOEmbed(t *testing.T) {
	resolver := &stubResolver{meta: &service.ExternalVideoMetadata{
		Title:        "Look",
		ThumbnailURL: "https://cdn/t.jpg",
		AuthorName:   "studio",
	}}
	svc := newExternalFeedSUT(t, resolver)

	res, err := svc.CreateFromExternal(staffActor, request.CreateExternalFeedPostRequest{
		ExternalURL: "https://www.tiktok.com/@studio/video/7312345678",
		Caption:     "Mirá este look",
	})
	require.NoError(t, err)

	assert.Equal(t, service.ProviderTikTok, res.Provider)
	assert.Equal(t, service.ProviderTikTok, resolver.gotProvider)
	assert.Equal(t, "https://www.tiktok.com/@studio/video/7312345678", res.ExternalURL)
	assert.Equal(t, "7312345678", res.ExternalID)
	assert.Equal(t, "https://cdn/t.jpg", res.ThumbnailURL)
	assert.Equal(t, "studio", res.ExternalAuthor)
	assert.Empty(t, res.VideoURL, "external posts never rehost: video_url stays empty")
	assert.True(t, res.IsActive)
	// A canonical link already carries its id, so no redirect hop is paid.
	assert.Empty(t, resolver.gotCanonical, "canonical url must not trigger a redirect chase")
}

// TestFeed_CreateExternal_ResolvesShortLink: a TikTok share/short link has no
// post id in its path, so the canonical URL is resolved first and the id is
// extracted from it; the canonical URL is what gets stored as the deep link.
func TestFeed_CreateExternal_ResolvesShortLink(t *testing.T) {
	resolver := &stubResolver{
		meta:      &service.ExternalVideoMetadata{Title: "Look", AuthorName: "studio"},
		canonical: "https://www.tiktok.com/@studio/video/7312345678",
	}
	svc := newExternalFeedSUT(t, resolver)

	res, err := svc.CreateFromExternal(staffActor, request.CreateExternalFeedPostRequest{
		ExternalURL: "https://vt.tiktok.com/ZSABC123/",
		Caption:     "Link corto",
	})
	require.NoError(t, err)

	assert.Equal(t, service.ProviderTikTok, res.Provider)
	assert.Equal(t, "https://vt.tiktok.com/ZSABC123/", resolver.gotCanonical)
	assert.Equal(t, "7312345678", res.ExternalID)
	assert.Equal(t, "https://www.tiktok.com/@studio/video/7312345678", res.ExternalURL)
}

// TestFeed_CreateExternal_ShortLinkRedirectOffProvider: a redirect that leaves
// the provider is ignored, so the stored link cannot be swapped for an
// unrelated host.
func TestFeed_CreateExternal_ShortLinkRedirectOffProvider(t *testing.T) {
	resolver := &stubResolver{canonical: "https://evil.test/@studio/video/7312345678"}
	svc := newExternalFeedSUT(t, resolver)

	res, err := svc.CreateFromExternal(staffActor, request.CreateExternalFeedPostRequest{
		ExternalURL: "https://vt.tiktok.com/ZSABC123/",
		Caption:     "Link corto",
	})
	require.NoError(t, err)

	assert.Equal(t, "https://vt.tiktok.com/ZSABC123/", res.ExternalURL)
	assert.Empty(t, res.ExternalID)
}

// TestFeed_CreateExternal_RejectsUnsupportedHost: an unsupported host is a hard
// error — the provider gate runs before anything is persisted.
func TestFeed_CreateExternal_RejectsUnsupportedHost(t *testing.T) {
	svc := newExternalFeedSUT(t, &stubResolver{})

	_, err := svc.CreateFromExternal(centerActor, request.CreateExternalFeedPostRequest{
		ExternalURL: "https://example.com/watch?v=1",
		Caption:     "nope",
	})
	assert.Error(t, err)
}

// TestFeed_CreateExternal_ResolverFailureStoresBareLink: a failed or
// unsupported (Instagram) lookup must NOT block the post — the bare link is
// still stored so the card renders client-side.
func TestFeed_CreateExternal_ResolverFailureStoresBareLink(t *testing.T) {
	svc := newExternalFeedSUT(t, &stubResolver{err: errors.New("oembed down")})

	res, err := svc.CreateFromExternal(centerActor, request.CreateExternalFeedPostRequest{
		ExternalURL: "https://www.instagram.com/reel/abc123/",
		Caption:     "reel",
	})
	require.NoError(t, err)
	assert.Equal(t, service.ProviderInstagram, res.Provider)
	assert.Equal(t, "abc123", res.ExternalID)
	assert.Empty(t, res.ThumbnailURL)
}

// TestExternalIDFromURL covers the provider-specific id extraction.
func TestExternalIDFromURL(t *testing.T) {
	cases := []struct {
		url      string
		expected string
	}{
		{"https://www.tiktok.com/@u/video/7312345678", "7312345678"},
		{"https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=5", "dQw4w9WgXcQ"},
		{"https://www.youtube.com/shorts/abcDEF", "abcDEF"},
		{"https://www.instagram.com/reel/abc123/", "abc123"},
		{"https://example.com/x", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, service.ExternalIDFromURL(c.url), c.url)
	}
}
