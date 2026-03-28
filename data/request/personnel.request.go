package request

type PersonnelImageRequest struct {
	ImageUrl string `json:"image_url" binding:"required,url"`
	IsCover  bool   `json:"is_cover"`
	Position int    `json:"position"`
}

type CreatePersonnelRequest struct {
	BeautyCenterID uint                    `json:"beauty_center_id" binding:"required"`
	FirstName      string                  `json:"first_name" binding:"required,max=50"`
	LastName       string                  `json:"last_name" binding:"required,max=50"`
	Email          string                  `json:"email" binding:"required,email,max=70"`
	Phone          string                  `json:"phone" binding:"required,max=20"`
	Password       string                  `json:"password" binding:"required,min=8,max=255"`
	IsActive       bool                    `json:"is_active"`
	Images         []PersonnelImageRequest `json:"images"`
}

type UpdatePersonnelRequest struct {
	FirstName string                  `json:"first_name" binding:"omitempty,max=50"`
	LastName  string                  `json:"last_name" binding:"omitempty,max=50"`
	Email     string                  `json:"email" binding:"omitempty,email,max=70"`
	Phone     string                  `json:"phone" binding:"omitempty,max=20"`
	Password  string                  `json:"password" binding:"omitempty,min=8,max=255"`
	IsActive  bool                    `json:"is_active"`
	Images    []PersonnelImageRequest `json:"images"`
}
