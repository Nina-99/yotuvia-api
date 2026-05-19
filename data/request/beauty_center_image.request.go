package request

type BeautyCenterImageRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	ImageUrl       string `json:"image_url" validate:"required"`
	IsCover        bool   `json:"is_cover"`
	Position       int    `json:"position"`
}
