package request

type BeautyCenterImageRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	ImageUrl       string `json:"image_url" validate:"required"`
	IsCover        bool   `json:"is_cover"`
	IsProfile      bool   `json:"is_profile"`
	Position       int    `json:"position"`
}

type UpdateBeautyCenterImageRequest struct {
	IsCover   *bool `json:"is_cover"`
	IsProfile *bool `json:"is_profile"`
	Position  *int  `json:"position"`
}

type ReorderImagesRequest struct {
	Order []ImageOrder `json:"order" validate:"required,dive"`
}

type ImageOrder struct {
	ID       uint `json:"id" validate:"required"`
	Position int  `json:"position" validate:"min=0"`
}
