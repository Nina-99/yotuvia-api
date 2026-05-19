package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/go-playground/validator/v10"
)

type ReviewServiceImpl struct {
	repo     repository.ReviewRepository
	Validate *validator.Validate
}

func NewReviewServiceImpl(repo repository.ReviewRepository, validate *validator.Validate) ReviewService {
	return &ReviewServiceImpl{repo: repo, Validate: validate}
}

func (s *ReviewServiceImpl) Create(clientID uint, req request.CreateReviewRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}
	review := &models.Review{
		ClientID:       clientID,
		BeautyCenterID: req.BeautyCenterID,
		Rating:         req.Rating,
		Comment:        req.Comment,
	}
	return s.repo.Create(review)
}

func (s *ReviewServiceImpl) Update(clientID uint, reviewID uint, req request.UpdateReviewRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}
	review, err := s.repo.FindByID(reviewID)
	if err != nil {
		return err
	}

	if review.ClientID != clientID {
		return errors.New("unauthorized: you can only update your own reviews")
	}

	if req.Rating != nil {
		review.Rating = *req.Rating
	}
	if req.Comment != nil {
		review.Comment = *req.Comment
	}

	return s.repo.Update(review)
}

func (s *ReviewServiceImpl) Delete(clientID uint, reviewID uint) error {
	review, err := s.repo.FindByID(reviewID)
	if err != nil {
		return err
	}

	if review.ClientID != clientID {
		return errors.New("unauthorized: you can only delete your own reviews")
	}

	return s.repo.Delete(reviewID)
}

func mapToResponse(review *models.Review) response.ReviewResponse {
	return response.ReviewResponse{
		ID:             review.ID,
		Rating:         review.Rating,
		Comment:        review.Comment,
		CreatedAt:      review.CreatedAt,
		BeautyCenterID: review.BeautyCenterID,
		ClientID:       review.ClientID,
		Client: response.ClientResponse{
			ID:        review.Client.ID,
			FirstName: review.Client.FirstName,
			LastName:  review.Client.LastName,
			Username:  review.Client.Username,
			Email:     review.Client.Email,
			Phone:     review.Client.Phone,
		},
		BeautyCenter: response.BeautyCenterResponse{
			ID:          review.BeautyCenter.ID,
			Name:        review.BeautyCenter.Name,
			Address:     review.BeautyCenter.Address,
			Email:       review.BeautyCenter.Email,
			Phone:       review.BeautyCenter.Phone,
			Url:         review.BeautyCenter.Url,
			Description: review.BeautyCenter.Description,
			IsActive:    review.BeautyCenter.IsActive,
		},
	}
}

func (s *ReviewServiceImpl) FindByID(id uint) (*response.ReviewResponse, error) {
	review, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	resp := mapToResponse(review)
	return &resp, nil
}

func (s *ReviewServiceImpl) FindByBeautyCenterID(beautyCenterID uint) ([]response.ReviewResponse, error) {
	reviews, err := s.repo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return nil, err
	}

	var res []response.ReviewResponse
	for _, r := range reviews {
		res = append(res, mapToResponse(&r))
	}
	return res, nil
}

func (s *ReviewServiceImpl) FindByClientID(clientID uint) ([]response.ReviewResponse, error) {
	reviews, err := s.repo.FindByClientID(clientID)
	if err != nil {
		return nil, err
	}

	var res []response.ReviewResponse
	for _, r := range reviews {
		res = append(res, mapToResponse(&r))
	}
	return res, nil
}
