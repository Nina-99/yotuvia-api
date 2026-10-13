package service

import (
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/repository"
)

type TreatmentServiceImpl struct {
	treatmentRepo repository.TreatmentRepository
}

func NewTreatmentServiceImpl(treatmentRepo repository.TreatmentRepository) TreatmentService {
	return &TreatmentServiceImpl{treatmentRepo: treatmentRepo}
}

// Catalog returns the active global vocabulary grouped by category. Empty
// categories are dropped so the client never renders a dead section.
func (s *TreatmentServiceImpl) Catalog() (response.TreatmentCatalogResponse, error) {
	categories, err := s.treatmentRepo.FindCatalog()
	if err != nil {
		return response.TreatmentCatalogResponse{}, err
	}

	out := response.TreatmentCatalogResponse{Categories: []response.TreatmentCategoryResponse{}}
	for _, c := range categories {
		if len(c.Treatments) == 0 {
			continue
		}
		entry := response.TreatmentCategoryResponse{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			Icon:        c.Icon,
			Treatments:  make([]response.TreatmentSummary, 0, len(c.Treatments)),
		}
		for _, t := range c.Treatments {
			entry.Treatments = append(entry.Treatments, response.TreatmentSummary{ID: t.ID, Name: t.Name})
			out.Total++
		}
		out.Categories = append(out.Categories, entry)
	}
	return out, nil
}
