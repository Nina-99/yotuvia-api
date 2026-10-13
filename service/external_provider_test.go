package service

import "testing"

func TestProviderFromURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"tiktok", "https://www.tiktok.com/@user/video/123", ProviderTikTok},
		{"tiktok mobile", "https://m.tiktok.com/v/123.html", ProviderTikTok},
		{"youtube watch", "https://www.youtube.com/watch?v=abc", ProviderYouTube},
		{"youtube short", "https://youtu.be/abc", ProviderYouTube},
		{"instagram reel", "https://www.instagram.com/reel/abc/", ProviderInstagram},
		{"empty", "", ""},
		{"not a url", "not a url", ""},
		{"unsupported host", "https://vimeo.com/123", ""},
		// Must not be fooled by a lookalike host.
		{"lookalike", "https://tiktok.com.evil.test/video/1", ""},
		{"subdomain lookalike", "https://notreally.tiktok.com.evil.test/x", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProviderFromURL(tc.url); got != tc.want {
				t.Fatalf("provider(%q): want %q, got %q", tc.url, tc.want, got)
			}
		})
	}
}

func TestIsSupportedExternalURL(t *testing.T) {
	if !IsSupportedExternalURL("https://www.tiktok.com/@u/video/1") {
		t.Fatal("tiktok url must be supported")
	}
	if IsSupportedExternalURL("https://example.com/v.mp4") {
		t.Fatal("unknown host must be rejected")
	}
}
