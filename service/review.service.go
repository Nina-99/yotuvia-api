package service

import (
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
)

type ReviewService interface {
	Create(clientID uint, req request.CreateReviewRequest) error
	Update(clientID uint, reviewID uint, req request.UpdateReviewRequest) error
	Delete(clientID uint, reviewID uint) error
	FindByID(id uint) (*response.ReviewResponse, error)
	FindByBeautyCenterID(beautyCenterID uint) ([]response.ReviewResponse, error)
	// FindByClientID is an authenticated read: a `client` token may read only
	// its own clientID (role + actor compare, 403 otherwise); beauty_center and
	// personnel keep inspecting any client's reviews.
	FindByClientID(clientID, actorClientID uint, role string) ([]response.ReviewResponse, error)
	CanReview(clientID, beautyCenterID uint) (bool, error)
}
