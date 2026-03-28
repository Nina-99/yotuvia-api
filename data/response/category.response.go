package response

type CategoryResponse struct {
	ID             uint   `json:"id"`
	BeautyCenterID uint   `json:"beauty_center_id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
}
