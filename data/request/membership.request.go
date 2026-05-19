package request

type CreateMembershipRequest struct {
	Name           string `json:"name" validate:"required,min=3,max=50"`
	Description    string `json:"description" validate:"max=255"`
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	IsActive       bool   `json:"is_active"`
}

type UpdateMembershipRequest struct {
	Name        string `json:"name" validate:"omitempty,min=3,max=50"`
	Description string `json:"description" validate:"omitempty,max=255"`
	IsActive    *bool  `json:"is_active"` // Pointer to distinguish between false and nil
}
