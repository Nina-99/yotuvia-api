package service

import (
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
)

// AvailabilityService backs the cross-center availability search
// (GET /availability/search) — "search by intent" over many centers and an
// inclusive date range, using the SAME slot kernel as the per-center
// endpoint so the two can never disagree.
type AvailabilityService interface {
	Search(req request.AvailabilitySearchRequest) (response.AvailabilitySearchResponse, error)
}
