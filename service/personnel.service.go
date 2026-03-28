package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type PersonnelService interface {
	Create(req request.CreatePersonnelRequest) (response.PersonnelResponse, error)
	Update(id uint, req request.UpdatePersonnelRequest) (response.PersonnelResponse, error)
	Delete(id uint) error
	FindByID(id uint) (response.PersonnelResponse, error)
	FindAll(beautyCenterID uint) ([]response.PersonnelResponse, error)
}
