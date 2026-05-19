package request

type CreateSocialMediaRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	Google         string `json:"google" validate:"omitempty,url"`
	Facebook       string `json:"facebook" validate:"omitempty,url"`
	Instagram      string `json:"instagram" validate:"omitempty,url"`
	Twitter        string `json:"twitter" validate:"omitempty,url"`
	TikTok         string `json:"tiktok" validate:"omitempty,url"`
}

type UpdateSocialMediaRequest struct {
	Google    string `json:"google" validate:"omitempty,url"`
	Facebook  string `json:"facebook" validate:"omitempty,url"`
	Instagram string `json:"instagram" validate:"omitempty,url"`
	Twitter   string `json:"twitter" validate:"omitempty,url"`
	TikTok    string `json:"tiktok" validate:"omitempty,url"`
}
