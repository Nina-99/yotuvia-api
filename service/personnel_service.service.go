package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type PersonnelServiceService interface {
	Create(req request.CreatePersonnelServiceRequest) (response.PersonnelServiceResponse, error)
	Delete(id uint) error
	FindByPersonnelID(personnelID uint) ([]response.PersonnelServiceResponse, error)
	FindByServiceID(serviceID uint) ([]response.PersonnelServiceResponse, error)
}
