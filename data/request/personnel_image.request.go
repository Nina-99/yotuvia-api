package request

type CreatePersonnelImageRequest struct {
	PersonnelID uint   `json:"personnel_id" validate:"required"`
	ImageUrl    string `json:"image_url" validate:"required,url"`
	IsCover     bool   `json:"is_cover"`
	IsProfile   bool   `json:"is_profile"`
	Position    int    `json:"position"`
}
