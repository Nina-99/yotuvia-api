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

type BuyMembershipRequest struct {
	MembershipID uint   `json:"membership_id" validate:"required"`
	ReceiptURL   string `json:"receipt_url" validate:"required,url"`
}

type ReviewMembershipRequest struct {
	Status string `json:"status" validate:"required,oneof=active rejected"`
}
