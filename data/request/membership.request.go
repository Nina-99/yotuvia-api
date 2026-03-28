package request

type CreateMembershipRequest struct {
	Name           string `json:"name" binding:"required,min=3,max=50"`
	Description    string `json:"description" binding:"max=255"`
	BeautyCenterID uint   `json:"beauty_center_id" binding:"required"`
	IsActive       bool   `json:"is_active"`
}

type UpdateMembershipRequest struct {
	Name        string `json:"name" binding:"omitempty,min=3,max=50"`
	Description string `json:"description" binding:"omitempty,max=255"`
	IsActive    *bool  `json:"is_active"` // Pointer to distinguish between false and nil
}
