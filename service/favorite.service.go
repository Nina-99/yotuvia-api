package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type FavoriteService interface {
	Create(req request.CreateFavoriteRequest) error
	Delete(id uint) error
	FindByClientID(clientID uint) ([]response.FavoriteResponse, error)
	IsFavorite(clientID uint, beautyCenterID uint) (bool, error)
}
