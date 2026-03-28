package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type ReviewService interface {
	Create(clientID uint, req request.CreateReviewRequest) error
	Update(clientID uint, reviewID uint, req request.UpdateReviewRequest) error
	Delete(clientID uint, reviewID uint) error
	FindByID(id uint) (*response.ReviewResponse, error)
	FindByBeautyCenterID(beautyCenterID uint) ([]response.ReviewResponse, error)
	FindByClientID(clientID uint) ([]response.ReviewResponse, error)
}
