package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type LoyaltyService interface {
	CreateProgram(req request.CreateLoyaltyProgrmaRequest) (response.LoyaltyProgramResponse, error)
	AddPunch(reservationID, clientID, beautyCenterID, serviceID uint) error
	GetClientsCards(clientID uint) ([]response.PunchCardResponse, error)
}
