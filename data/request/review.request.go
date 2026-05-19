package request

type CreateReviewRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	Rating         int    `json:"rating" validate:"required,min=1,max=5"`
	Comment        string `json:"comment"`
}

type UpdateReviewRequest struct {
	Rating  *int    `json:"rating" validate:"omitempty,min=1,max=5"`
	Comment *string `json:"comment"`
}
