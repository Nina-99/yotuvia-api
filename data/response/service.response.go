package response

type ServiceResponse struct {
	ID             uint    `json:"id"`
	BeautyCenterID uint    `json:"beauty_center_id"`
	CategoryID     uint    `json:"category_id"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	// Raw per-locale maps (owner edit forms prefill these). `name`/`description`
	// above are resolved for the request's `lang`.
	NameI18n        map[string]string `json:"name_i18n,omitempty"`
	DescriptionI18n map[string]string `json:"description_i18n,omitempty"`
	Duration       int     `json:"duration"`
	Price          float64 `json:"price"`
	ImageURL       string  `json:"image_url"`
	IsActive       bool    `json:"is_active"`
	// TreatmentID is the nullable link to the global treatment vocabulary.
	TreatmentID *uint            `json:"treatment_id,omitempty"`
	Category    CategoryResponse `json:"category,omitempty"`
}
