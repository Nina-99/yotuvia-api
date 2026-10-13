package service

import "github.com/Nina-99/yotuvia-api/data/response"

// TreatmentService exposes the GLOBAL treatment vocabulary to clients and to
// the center's service editor.
type TreatmentService interface {
	Catalog() (response.TreatmentCatalogResponse, error)
}
