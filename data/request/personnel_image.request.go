package request

type CreatePersonnelImageRequest struct {
	PersonnelID uint   `json:"personnel_id" validate:"required"`
	ImageUrl    string `json:"image_url" validate:"required,url"`
	IsCover     bool   `json:"is_cover"`
	Position    int    `json:"position"`
}
