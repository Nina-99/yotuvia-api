package request

type CreateSocialMediaRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" binding:"required"`
	Google         string `json:"google" binding:"omitempty,url"`
	Facebook       string `json:"facebook" binding:"omitempty,url"`
	Instagram      string `json:"instagram" binding:"omitempty,url"`
	Twitter        string `json:"twitter" binding:"omitempty,url"`
	TikTok         string `json:"tiktok" binding:"omitempty,url"`
}

type UpdateSocialMediaRequest struct {
	Google    string `json:"google" binding:"omitempty,url"`
	Facebook  string `json:"facebook" binding:"omitempty,url"`
	Instagram string `json:"instagram" binding:"omitempty,url"`
	Twitter   string `json:"twitter" binding:"omitempty,url"`
	TikTok    string `json:"tiktok" binding:"omitempty,url"`
}
