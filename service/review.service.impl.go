package service

import (
	"errors"
	"fmt"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type ReviewServiceImpl struct {
	repo                repository.ReviewRepository
	notificationService NotificationService
	missionService      TokenMissionService
	Validate            *validator.Validate
}

func NewReviewServiceImpl(repo repository.ReviewRepository, notificationService NotificationService, missionService TokenMissionService, validate *validator.Validate) ReviewService {
	return &ReviewServiceImpl{repo: repo, notificationService: notificationService, missionService: missionService, Validate: validate}
}

func (s *ReviewServiceImpl) Create(clientID uint, req request.CreateReviewRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}

	// Strict verification: only clients with a COMPLETED reservation can review
	hasCompleted, err := s.repo.HasCompletedReservation(clientID, req.BeautyCenterID)
	if err != nil {
		return errors.New("error verifying reservation")
	}
	if !hasCompleted {
		return errors.New("debes completar un turno antes de dejar una reseña")
	}

	// Prevent duplicate reviews
	existing, err := s.repo.FindByClientID(clientID)
	if err != nil {
		return err
	}
	for _, r := range existing {
		if r.BeautyCenterID == req.BeautyCenterID {
			return errors.New("ya dejaste una reseña para este centro")
		}
	}

	review := &models.Review{
		ClientID:       clientID,
		BeautyCenterID: req.BeautyCenterID,
		Rating:         req.Rating,
		Comment:        req.Comment,
	}
	if err := s.repo.Create(review); err != nil {
		return err
	}

	// Best-effort trigger (design D4): the center is told a client reviewed it.
	// Only fires after the insert succeeded — a failed or duplicate review
	// emits nothing — and never fails the business write.
	_ = s.notificationService.CreateForCenter(review.BeautyCenterID, "review_created",
		"Nueva reseña",
		fmt.Sprintf("Recibiste una reseña de %d estrellas", review.Rating),
		fmt.Sprintf(`{"review_id":%d,"rating":%d}`, review.ID, review.Rating))

	// Best-effort mission trigger (feat-token-missions D6): review_received.
	if s.missionService != nil {
		if err := s.missionService.TriggerCheck(review.BeautyCenterID, "review_received"); err != nil {
			utils.Log.Errorf("[MISSION] review_received trigger failed for center %d: %v",
				review.BeautyCenterID, err)
		}
	}
	return nil
}

func (s *ReviewServiceImpl) CanReview(clientID, beautyCenterID uint) (bool, error) {
	// Check completed reservation
	hasCompleted, err := s.repo.HasCompletedReservation(clientID, beautyCenterID)
	if err != nil || !hasCompleted {
		return false, err
	}
	// Check no existing review
	existing, err := s.repo.FindByClientID(clientID)
	if err != nil {
		return false, err
	}
	for _, r := range existing {
		if r.BeautyCenterID == beautyCenterID {
			return false, nil
		}
	}
	return true, nil
}

func (s *ReviewServiceImpl) Update(clientID uint, reviewID uint, req request.UpdateReviewRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}
	review, err := s.repo.FindByID(reviewID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrNotFound
		}
		return err
	}

	if review.ClientID != clientID {
		return apperrors.ErrForbidden
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrNotFound
		}
		return err
	}

	if review.ClientID != clientID {
		return apperrors.ErrForbidden
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
		Client: response.ReviewClientResponse{
			ID:        review.Client.ID,
			FirstName: review.Client.FirstName,
			LastName:  review.Client.LastName,
			AvatarURL: review.Client.AvatarURL,
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

func (s *ReviewServiceImpl) FindByClientID(clientID, actorClientID uint, role string) ([]response.ReviewResponse, error) {
	// Ownership: a `client` may read only its own reviews. beauty_center and
	// personnel keep inspecting any client's reviews — that is a legitimate
	// staff-facing view, not an IDOR. Mirrors the loyalty punch-card rule.
	if role == "client" && actorClientID != clientID {
		return nil, apperrors.ErrForbidden
	}

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
