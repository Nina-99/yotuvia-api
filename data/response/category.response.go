package response

type CategoryResponse struct {
	ID             uint   `json:"id"`
	BeautyCenterID uint   `json:"beauty_center_id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	NameI18n        map[string]string `json:"name_i18n,omitempty"`
	DescriptionI18n map[string]string `json:"description_i18n,omitempty"`
	Icon           string `json:"icon"`
}
