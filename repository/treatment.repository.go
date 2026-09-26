package repository

import "github.com/Nina-99/yottu-api/models"

// TreatmentRepository is the read side of the GLOBAL treatment vocabulary.
type TreatmentRepository interface {
	// FindCatalog returns every active treatment category with its active
	// treatments preloaded, ordered for stable presentation.
	FindCatalog() ([]models.TreatmentCategory, error)
	// FindByIDs loads the active treatments for the given IDs (used to reject
	// unknown treatment filters on the availability search).
	FindByIDs(ids []uint) ([]models.Treatment, error)
}
