package response

type MembershipResponse struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	BeautyCenterID uint   `json:"beauty_center_id"`
	IsActive       bool   `json:"is_active"`
}
