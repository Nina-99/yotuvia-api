package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type MembershipService interface {
	Create(req request.CreateMembershipRequest) error
	Update(id uint, req request.UpdateMembershipRequest) error
	Delete(id uint) error
	FindByID(id uint) (response.MembershipResponse, error)
	FindAll() ([]response.MembershipResponse, error)
}
