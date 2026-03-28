package response

type FavoriteResponse struct {
	ID             uint `json:"id"`
	ClientID       uint `json:"client_id"`
	BeautyCenterID uint `json:"beauty_center_id"`
	BeautyCenter   struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
		// Other fields can be added here
	} `json:"beauty_center"`
}
