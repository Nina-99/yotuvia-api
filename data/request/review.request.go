package request

type CreateReviewRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" binding:"required"`
	Rating         int    `json:"rating" binding:"required,min=1,max=5"`
	Comment        string `json:"comment"`
}

type UpdateReviewRequest struct {
	Rating  *int    `json:"rating" binding:"omitempty,min=1,max=5"`
	Comment *string `json:"comment"`
}
