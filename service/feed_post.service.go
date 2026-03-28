package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type FeedPostService interface {
	Create(req request.CreateFeedPostRequest) (response.FeedPostResponse, error)
	Update(id uint, req request.UpdateFeedPostRequest) (response.FeedPostResponse, error)
	Delete(id uint) error
	FindByID(id uint) (response.FeedPostResponse, error)
	FindAll() ([]response.FeedPostResponse, error)
	FindByBeautyCenterID(centerID uint) ([]response.FeedPostResponse, error)
}
