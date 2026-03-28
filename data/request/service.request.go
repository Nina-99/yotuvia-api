package request

type CreateServiceRequest struct {
	BeautyCenterID uint    `json:"beauty_center_id" validate:"required"`
	CategoryID     uint    `json:"category_id" validate:"required"`
	Name           string  `json:"name" validate:"required,min=2,max=100"`
	Description    string  `json:"description" validate:"required"`
	Duration       int     `json:"duration" validate:"required,gt=0"`
	Price          float64 `json:"price" validate:"required,gte=0"`
	ImageURL       string  `json:"image_url"`
	IsActive       bool    `json:"is_active"`
}

type UpdateServiceRequest struct {
	BeautyCenterID uint    `json:"beauty_center_id"`
	CategoryID     uint    `json:"category_id"`
	Name           string  `json:"name" validate:"omitempty,min=2,max=100"`
	Description    string  `json:"description"`
	Duration       int     `json:"duration" validate:"omitempty,gt=0"`
	Price          float64 `json:"price" validate:"omitempty,gte=0"`
	ImageURL       string  `json:"image_url"`
	IsActive       *bool   `json:"is_active"`
}
