package service

import (
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
)

type PersonnelServiceService interface {
	// Create assigns a service to a professional. actorCenterID is the acting
	// centre's JWT claim: both the personnel and the service must belong to it,
	// otherwise the write is rejected with ErrForbidden.
	Create(req request.CreatePersonnelServiceRequest, actorCenterID uint) (response.PersonnelServiceResponse, error)
	// Delete removes an assignment. actorCenterID must own the assignment's
	// personnel and service, otherwise ErrForbidden.
	Delete(id uint, actorCenterID uint) error
	FindByPersonnelID(personnelID uint) ([]response.PersonnelServiceResponse, error)
	FindByServiceID(serviceID uint) ([]response.PersonnelServiceResponse, error)
}
