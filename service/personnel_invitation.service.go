package service

import (
	"github.com/Nina-99/yottu-api/data/response"
)

type PersonnelInvitationService interface {
	Generate(beautyCenterID uint, email string) (string, error)
	GetByToken(token string) (*response.InvitationResponse, error)
	Accept(token string, firstName, lastName, phone, password string) error
	ListByCenter(beautyCenterID uint) ([]response.InvitationResponse, error)
}
