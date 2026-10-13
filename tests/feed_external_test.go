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
}

func (s *stubResolver) Resolve(_ context.Context, _ string, provider string) (*service.ExternalVideoMetadata, error) {
	s.gotProvider = provider
	return s.meta, s.err
}

func newExternalFeedSUT(t *testing.T, resolver service.OEmbedResolver) service.FeedPostService {
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
