package request

type CreateCategoryRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	Name           string `json:"name" validate:"required,min=3"`
	Description    string `json:"description"`
	NameI18n        map[string]string `json:"name_i18n"`
	DescriptionI18n map[string]string `json:"description_i18n"`
	Icon           string `json:"icon"`
}
