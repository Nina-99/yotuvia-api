package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type BeautyCenterService interface {
	Update(id uint, req request.UpdateBeautyCenterRequest) (response.BeautyCenterResponse, error)
	Delete(id uint) error
	FindByID(id uint) (response.BeautyCenterResponse, error)
	FindAll(page, limit int) ([]response.BeautyCenterResponse, int64, error)
}
