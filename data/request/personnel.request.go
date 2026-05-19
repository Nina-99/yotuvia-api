package request

type PersonnelImageRequest struct {
	ImageUrl string `json:"image_url" validate:"required,url"`
	IsCover  bool   `json:"is_cover"`
	Position int    `json:"position"`
}

type CreatePersonnelRequest struct {
	BeautyCenterID uint                    `json:"beauty_center_id" validate:"required"`
	FirstName      string                  `json:"first_name" validate:"required,max=50"`
	LastName       string                  `json:"last_name" validate:"required,max=50"`
	Email          string                  `json:"email" validate:"required,email,max=70"`
	Phone          string                  `json:"phone" validate:"required,max=20"`
	Password       string                  `json:"password" validate:"required,min=8,max=255"`
	IsActive       bool                    `json:"is_active"`
	Images         []PersonnelImageRequest `json:"images"`
}

type UpdatePersonnelRequest struct {
	FirstName string                  `json:"first_name" validate:"omitempty,max=50"`
	LastName  string                  `json:"last_name" validate:"omitempty,max=50"`
	Email     string                  `json:"email" validate:"omitempty,email,max=70"`
	Phone     string                  `json:"phone" validate:"omitempty,max=20"`
	Password  string                  `json:"password" validate:"omitempty,min=8,max=255"`
	IsActive  bool                    `json:"is_active"`
	Images    []PersonnelImageRequest `json:"images"`
}
