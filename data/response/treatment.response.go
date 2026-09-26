package response

// TreatmentSummary is one entry of the global treatment vocabulary as returned
// by GET /treatments.
type TreatmentSummary struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// TreatmentCategoryResponse groups the vocabulary under a global category.
type TreatmentCategoryResponse struct {
	ID          uint               `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Icon        string             `json:"icon"`
	Treatments  []TreatmentSummary `json:"treatments"`
}

// TreatmentCatalogResponse is the GET /treatments envelope: the whole global
// catalog, grouped by category, plus the total number of treatments.
type TreatmentCatalogResponse struct {
	Categories []TreatmentCategoryResponse `json:"categories"`
	Total      int                         `json:"total"`
}
