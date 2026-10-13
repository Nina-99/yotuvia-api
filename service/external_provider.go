package service

import (
	"net/url"
	"strings"
)

// External video providers supported by the unified feed. A native upload uses
// ProviderNative; external posts deep-link to the source and are never rehosted.
const (
	ProviderNative    = "native"
	ProviderTikTok    = "tiktok"
	ProviderYouTube   = "youtube"
	ProviderInstagram = "instagram"
)

// ProviderFromURL detects the external provider from a video URL, or returns ""
// when the URL is empty, malformed or from an unsupported host. It is the
// validation gate for linking an external video, so it must be strict about the
// host (a substring match like "tiktok.com.evil.test" must NOT pass).
func ProviderFromURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case hostMatches(host, "tiktok.com"):
		return ProviderTikTok
	case hostMatches(host, "youtube.com") || host == "youtu.be":
		return ProviderYouTube
	case hostMatches(host, "instagram.com"):
		return ProviderInstagram
	default:
		return ""
	}
}

// hostMatches reports whether `host` is exactly `domain` or a subdomain of it
// (e.g. "www.tiktok.com" matches "tiktok.com" but "tiktok.com.evil.test" does
// not).
func hostMatches(host, domain string) bool {
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// IsSupportedExternalURL reports whether the URL can be attached as an external
// feed item.
func IsSupportedExternalURL(rawURL string) bool {
	return ProviderFromURL(rawURL) != ""
}

// ExternalIDFromURL extracts a provider-specific video id from a supported URL,
// or "" when it cannot be derived. Best-effort: used for dedup and for building
// the official embed later, never as a validation gate.
func ExternalIDFromURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	switch ProviderFromURL(rawURL) {
	case ProviderTikTok:
		// /@user/video/<id>
		for i := 0; i+1 < len(segments); i++ {
			if segments[i] == "video" {
				return segments[i+1]
			}
		}
	case ProviderYouTube:
		if id := parsed.Query().Get("v"); id != "" {
			return id
		}
		// youtu.be/<id>, /shorts/<id>, /embed/<id>
		if parsed.Hostname() == "youtu.be" && len(segments) > 0 {
			return segments[0]
		}
		for i := 0; i+1 < len(segments); i++ {
			if segments[i] == "shorts" || segments[i] == "embed" {
				return segments[i+1]
			}
		}
	case ProviderInstagram:
		// /reel/<code>/ or /p/<code>/
		for i := 0; i+1 < len(segments); i++ {
			if segments[i] == "reel" || segments[i] == "p" || segments[i] == "tv" {
				return segments[i+1]
			}
		}
	}
	return ""
}
