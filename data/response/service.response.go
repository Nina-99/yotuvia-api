package response

type ServiceResponse struct {
	ID             uint             `json:"id"`
	BeautyCenterID uint             `json:"beauty_center_id"`
	CategoryID     uint             `json:"category_id"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	Duration       int              `json:"duration"`
	Price          float64          `json:"price"`
	ImageURL       string           `json:"image_url"`
	IsActive       bool             `json:"is_active"`
	Category       CategoryResponse `json:"category,omitempty"`
}
