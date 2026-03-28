package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type BeautyCenterService interface {
	Create(req request.CreateBeautyCenterRequest) (response.BeautyCenterResponse, error)
	Update(id uint, req request.UpdateBeautyCenterRequest) (response.BeautyCenterResponse, error)
	Delete(id uint) error
	FindByID(id uint) (response.BeautyCenterResponse, error)
	FindAll() ([]response.BeautyCenterResponse, error)
}
