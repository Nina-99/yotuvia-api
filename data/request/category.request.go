package request

type CreateCategoryRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	Name           string `json:"name" validate:"required,min=3"`
	Description    string `json:"description"`
}
