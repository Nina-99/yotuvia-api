package response

// TikTokAccountResponse is the connected-account summary. Tokens are never
// included: they never leave the backend. Enabled reports whether the whole
// integration is configured, so the UI can hide the connect action when the
// TikTok app is not (yet) set up.
type TikTokAccountResponse struct {
	Enabled     bool   `json:"enabled"`
	Connected   bool   `json:"connected"`
	DisplayName string `json:"display_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// TikTokVideoResponse is one of the connected user's own TikTok videos.
type TikTokVideoResponse struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Duration      int    `json:"duration"`
	CoverImageURL string `json:"cover_image_url"`
	ShareURL      string `json:"share_url"`
}
